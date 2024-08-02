package status_check

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	clientv3 "go.etcd.io/etcd/client/v3"
	"os"
	"path/filepath"
	"strings"
	"system-usability-detection/internal/config"
	"system-usability-detection/internal/util"
	"system-usability-detection/pkg/metrics"
	"time"
)

var _ StatusInterface = (*PowerCacheImpl)(nil)

// PowerCacheImpl powercache 服务检测
type PowerCacheImpl struct {
	MountPoint string //  /var/powercache
}

func (p *PowerCacheImpl) Name() string {
	return "power_cache"
}
func (p *PowerCacheImpl) CheckStatus() StatusAction {
	util.Logger.Info("powerCacheDisable:%v ,hasAvailablePowerCache:%v", powerCacheDisable, hasAvailablePowerCache)
	metrics.CacheCheckCounter.WithLabelValues("total").Inc()
	sa := StatusAction{
		Time:   time.Now(),
		Name:   p.Name(),
		Status: false,
	}
	if !powerCacheDisable {
		metrics.CacheCheckCounter.WithLabelValues("failed").Inc()
		sa.Status = !powerCacheDisable
		return sa
	}
	//当前节点的power不可用,需要检查其他节点是否有可用的power_cache
	if hasAvailablePowerCache {
		metrics.CacheCheckCounter.WithLabelValues("failed").Inc()
		//可以切换VIP
		return sa
	}
	//没有可用的power_cache,切换VIP没有意义
	sa.Status = true
	return sa
}

// 检测挂载点是否存在
func checkMountPoint(mountPoint string) (exist bool, err error) {
	now := time.Now()
	util.Logger.Info("start checkMountPoint at:%v", now.Unix())
	defer func() {
		used := time.Since(now)
		util.Logger.Info("checkMountPoint used:%v", used)
	}()
	file, err := os.Open("/proc/mounts")
	if err != nil {
		return
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		if strings.Contains(scanner.Text(), mountPoint) {
			return true, nil
		}
	}
	if err = scanner.Err(); err != nil {
		return
	}
	return
}

// 标记power_cache是否可用,true 表示不能用 false 表示可用
var (
	powerCacheDisable bool

	// 当powerCacheDisable为true的时候,需要判断该值，如果该值为true:说明有其他节点的power_cache可用,可以切换
	// false:说明power_cache都不可用,此时切换VIP没有任何意义
	hasAvailablePowerCache = true

	// 通知AggregationPower函数执行聚合
	notifyAggregationPower = make(chan struct{}, 1)
	powerTimeOut           = 25 * time.Second
	//工作队列
	workerQueueCh = make(chan *resultFlag, 1)
	//结果队列
	resultCh = make(chan *resultFlag, 1)
)

type resultFlag struct {
	flag int64
	used time.Duration
	err  error
}

func checkPowerCache(mountPoint string, filePath string) {
	_, err := os.Create(filePath)
	if err != nil {
		util.Logger.Error("create file failed:%v", err)
	}
	for rf := range workerQueueCh {
		//开始新一轮检测
		go func() {
			var (
				fileHandler *os.File
				err         error
			)
			newRF := &resultFlag{
				flag: rf.flag,
			}
			defer func() {
				if fileHandler != nil {
					fileHandler.Close()
				}
				newRF.used = time.Since(time.Unix(rf.flag, 0))
				resultCh <- newRF
			}()
			exist, err := checkMountPoint(mountPoint)
			//如果挂载点不存在
			if err != nil {
				newRF.err = err
				util.Logger.Error("power_cache check mountpoint failed:%v", err)
				return
			}
			if !exist {
				newRF.err = errors.New("mount point not exist")
				return
			}
			util.Logger.Info("start create or trunc at:%v", time.Now().Unix())
			//fileHandler, err = os.OpenFile(filePath, os.O_RDWR|os.O_TRUNC|syscall.O_DIRECT, 0666)
			fileHandler, err = os.OpenFile(filePath, os.O_RDWR|os.O_TRUNC, 0666)
			if err != nil {
				if os.IsNotExist(err) {
					//文件不存在，说明power_cache被清理过
					_, errCreate := os.Create(filePath)
					if errCreate != nil {
						util.Logger.Error("create file failed:%v", err)
					}
				}
				newRF.err = err
				util.Logger.Error("power_cache create or trunc failed:%v", err)
				return
			}
			util.Logger.Info("start write at:%v", time.Now().Unix())
			if _, err = fileHandler.WriteString(fmt.Sprintf("%v", rf.flag)); err != nil {
				newRF.err = err
				util.Logger.Error("power_cache write failed:%v", err)
				return
			}
		}()
	}
}

func backGroundPowerCheck(cli *clientv3.Client, mountPoint string) {
	name, _ := os.Hostname()
	var (
		writeFile = filepath.Join(mountPoint, ".write_check_"+name)
		count     int
	)
	//后台检测power_cache写操作
	go checkPowerCache(mountPoint, writeFile)
	workerQueueCh <- &resultFlag{
		flag: time.Now().Unix(),
	}
	for {
		select {
		case res := <-resultCh:
			//如果结果能读出来，再添加任务
			workerQueueCh <- &resultFlag{
				flag: time.Now().Unix(),
			}
			if res.err != nil {
				util.Logger.Error("execute power_cache check failed info:%v", res.err)
				count++
			} else {
				powerCacheDisable = false
				cleanCurrentPowerFromEtcd(cli)
				count = 0
			}

		case <-time.After(powerTimeOut):
			util.Logger.Error("execute power_cache check timeout more than:%v", powerTimeOut)
			count++
		}
		if count >= 5 {
			powerCacheDisable = true
			count = 0
			//当前的power不可用，推送到etcd
			pushCurrentPowerToEtcd(cli)
		}
		time.Sleep(5 * time.Second)
	}
}

// /power_cache/localip ---> time.Now().String()
var powerPrefix = "/disable_power_cache/"

// 从etcd把当前节点的power_cache移除掉
func cleanCurrentPowerFromEtcd(cli *clientv3.Client) {
	defer func() {
		notifyAggregationPower <- struct{}{}
	}()
	key := powerPrefix + config.GlobalConfigInstance.VrrpInstances.Instances[0].LocalIP
	util.Logger.Info("enable power cache key:%s", key)
	txn := cli.Txn(context.Background())
	//如果存在,移除
	txn.If(clientv3.Compare(clientv3.CreateRevision(key), "!=", 0)).
		Then(clientv3.OpDelete(key)).Else()
	//提交事务
	_, err := txn.Commit()
	if err != nil {
		util.Logger.Error("cleanCurrentPowerFromEtcd transcation commit failed:%v", err)
	}
}

// 把当前不可用的power_cache推送到etcd
func pushCurrentPowerToEtcd(cli *clientv3.Client) {
	defer func() {
		notifyAggregationPower <- struct{}{}
	}()
	key := powerPrefix + config.GlobalConfigInstance.VrrpInstances.Instances[0].LocalIP
	util.Logger.Info("disable power cache key:%s", key)
	txn := cli.Txn(context.Background())
	//如果不存在,新增
	txn.If(clientv3.Compare(clientv3.CreateRevision(key), "=", 0)).
		Then(clientv3.OpPut(key, time.Now().String())).Else()
	//提交事务
	_, err := txn.Commit()
	if err != nil {
		util.Logger.Error("pushCurrentPowerToEtcd transcation commit failed:%v", err)
	}
}

// 聚合etcd中power_cache的结果，更新hasAvailablePowerCache
func aggregationPower(cli *clientv3.Client) {
	ctx := context.Background()
	for range notifyAggregationPower {
		ctx, _ := context.WithTimeout(ctx, 2*time.Second)
		resp, err := cli.Get(ctx, powerPrefix, clientv3.WithPrefix())
		if err != nil {
			//如果此处被cancel掉,说明超时了
			util.Logger.Error("aggregationPower get etcd failed:%v", err)
			//etcd获取失败,此种情况下，我们认为其他节点的power都是可用的
			if !hasAvailablePowerCache {
				hasAvailablePowerCache = true
			}
			continue
		}
		if len(resp.Kvs) >= config.GlobalConfigInstance.InstancesCount {
			if hasAvailablePowerCache {
				hasAvailablePowerCache = false
			}
		} else {
			if !hasAvailablePowerCache {
				hasAvailablePowerCache = true
			}
		}
	}

}
