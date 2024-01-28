package server

import (
	"net/http"
	"strconv"
	"sync"
	"time"
)

type BrainServer struct {
	pubSubSystem *PubSub
	cli          *etcdClient
	subCh        chan interface{}
}

func NewBrainServer() (*BrainServer, error) {
	b := &BrainServer{
		pubSubSystem: New(),
		subCh:        make(chan interface{}, 1000),
	}
	cli, err := newEtcdClient(globalVrrpInstances.etcdPoints, globalVrrpInstances.dial, globalVrrpInstances.ttl)
	if err != nil {
		return nil, err
	}
	b.cli = cli
	return b, nil
}

func (b *BrainServer) Start(ctx context.Context, status []StatusInterface) {
	go b.cli.startKeepAlived(ctx)
	go b.pubKeepalivedServerStatus(ctx, status)
	go b.subKeepalivedServerStatus(ctx)
}

// curl -sL -m 1 -H 'Vip: 10.1.33.133' -H 'Local: enp101s0f1' -w %{http_code} http://10.1.33.45:12345/check -o /dev/null
func (b *BrainServer) BrainCheckHandler(w http.ResponseWriter, r *http.Request) {
	now := time.Now()
	var httpCode = http.StatusOK

	defer func() {
		used := time.Since(now)
		metrics.RequestHistogram.WithLabelValues(fmt.Sprintf("%d", httpCode), "GET").Observe(used.Seconds())
		logger.Infof("BrainCheckHandler used:%v", used)
	}()

	local := r.Header.Get("Local")
	if local == "" {
		httpCode = http.StatusBadRequest
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	// logger.Infof("local:%s", local)
	ip, err := getIPByName(local)
	if err != nil {
		logger.Errorf("get ip by interface name failed:%v", err)
		httpCode = http.StatusBadRequest
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	vip := r.Header.Get("Vip")
	if vip == "" {
		httpCode = http.StatusBadRequest
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	prefix := keepAlivedPrefix + vip
	key := prefix + "/" + ip
	resp, err := b.cli.get(context.Background(), prefix)
	if err != nil {
		logger.Errorf("get key %s with prefix failed:%v", prefix, err)
		httpCode = http.StatusInternalServerError
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if resp.Count == 0 {
		httpCode = http.StatusInternalServerError
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	var (
		exist    bool
		priority int
		max      = -256
	)
	for i := range resp.Kvs {
		k := resp.Kvs[i].Key
		v := resp.Kvs[i].Value
		v_priority, err := strconv.Atoi(string(v))
		if err != nil {
			continue
		}
		if key == string(k) {
			priority = v_priority
			exist = true
		}
		if v_priority >= max {
			max = v_priority
		}
	}
	if exist && priority == max {
		// 说明当前节点优先级最高
		w.WriteHeader(http.StatusOK)
		return
	}
	// httpCode = http.StatusForbidden
	w.WriteHeader(http.StatusForbidden)
}

// 订阅keepalived服务状态
func (b *BrainServer) subKeepalivedServerStatus(ctx context.Context) {
	b.pubSubSystem.Subscribe(b.subCh, ctx.Done(), func(entry interface{}) bool {
		_, ok := entry.([]StatusAction)
		return ok
	})
	for {
		select {
		case status := <-b.subCh:
			sa, ok := status.([]StatusAction)
			if !ok {
				continue
			}
			// 聚合状态查询
			var isOk = true
			for _, ele := range sa {
				if !ele.Status {
					isOk = false
					logger.Warningf("get status:%v failed", sa)
					break
				}
			}
			// 如果检查不是running状态
			if !isOk {
				notifyDown.close()
				continue
			}
			// running状态,到这还需要判断之前是否关闭过
			if notifyDown.isClosed() {
				notifyDown.renew()
				logger.Infof("get status ok, start keep alive again.")
				go b.cli.startKeepAlived(ctx)
			}

		case <-ctx.Done():
			logger.Warning("subKeepalivedServerStatus cancel all context")
			return
		}
	}
}

// 进入该函数之前,statusCheck已对重复Name进行拦截,获取keepalived服务状态,推送
func (b *BrainServer) pubKeepalivedServerStatus(ctx context.Context, statusCheck []StatusInterface) {
	var (
		oncePower sync.Once
		onceNas   sync.Once
	)
	var timeTicker = time.NewTimer(5 * time.Second)
	defer timeTicker.Stop()
	for {
		select {
		case <-timeTicker.C:
			timeTicker.Reset(5 * time.Second)
			result := make(chan []StatusAction, 1)
			go func() {
				var sts = make([]StatusAction, len(statusCheck))
				var wg sync.WaitGroup
				for i := range statusCheck {
					if statusCheck[i].Name() == "power_cache" {
						// 启动一个后台power_cache检测任务
						oncePower.Do(func() {
							point := statusCheck[i].(*PowerCacheImpl).MountPoint
							go backGroundPowerCheck(b.cli.cli, point)
							go aggregationPower(b.cli.cli)
						})
					}

					if statusCheck[i].Name() == "nas" {
						onceNas.Do(func() {
							addr := statusCheck[i].(*NasImpl).Address
							go backGroundNasCheck(addr)
						})
					}

					wg.Add(1)
					go func(index int) {
						defer wg.Done()
						sts[index] = statusCheck[index].CheckStatus()
					}(i)
				}
				wg.Wait()
				result <- sts
			}()
			select {
			case chanResult := <-result:
				metrics.ExecuteTimeOutGauge.Set(0)
				b.pubSubSystem.Publish(chanResult)
			case <-time.After(5 * time.Second):
				metrics.ExecuteTimeOutGauge.Set(1)
				b.pubSubSystem.Publish([]StatusAction{
					{
						Time:   time.Now(),
						Status: false,
						Extra:  "execute all check timeout 5 seconds",
					},
				})
				logger.Error("pubKeepalivedServerStatus execute all check timeout 5 seconds")
			}

		case <-ctx.Done():
			logger.Warning("pubKeepalivedServerStatus cancel all context")
			return
		}
	}
}
