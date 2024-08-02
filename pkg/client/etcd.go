package client

import (
	"context"
	"log"
	"sync"
	"time"

	"system-usability-detection/internal/config"
	"system-usability-detection/internal/util"

	clientv3 "go.etcd.io/etcd/client/v3"
)

type EtcdClient struct {
	cli            *clientv3.Client
	ttl, leaseTime int
}

func NewEtcdClient(endpoints []string, dial, ttl int) (*EtcdClient, error) {
	// TODO 和官方库不一致
	cfg := clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: time.Duration(dial) * time.Second,
		// LoadBalanceType
		// HealthCheckInterval
		DialKeepAliveTime:    10 * time.Second,
		DialKeepAliveTimeout: 2 * time.Second,
		PermitWithoutStream:  true,
	}
	cli, err := clientv3.New(cfg)
	if err != nil {
		return nil, err
	}
	return &EtcdClient{
		cli:       cli,
		ttl:       ttl,
		leaseTime: ttl + 1,
	}, nil
}

func (e *EtcdClient) get(ctx context.Context, key string) (*clientv3.GetResponse, error) {
	getCtx, cancel := context.WithTimeout(ctx, time.Duration(e.ttl)*time.Second)
	defer cancel()
	return e.cli.Get(getCtx, key, clientv3.WithPrefix())
}

func (e *EtcdClient) register(ctx context.Context, index int) error {
	leaseResp, err := e.cli.Grant(ctx, int64(e.leaseTime))
	if err != nil {
		// TODO 配置logger
		log.Printf("get lease failed: %v", err)
		return err
	}
	config.GlobalConfigInstance.VrrpInstances.Instances[index].LeaseID = leaseResp.ID
	if config.GlobalConfigInstance.VrrpInstances.Instances[index].KeepAliveCh, err = e.cli.KeepAlive(ctx, leaseResp.ID); err != nil {
		log.Printf("keepalive failed: %v", err)
		return err
	}
	key, val := config.GlobalConfigInstance.VrrpInstances.Instances[index].GenerateKV()
	if _, err := e.cli.Put(ctx, key, val, clientv3.WithLease(config.GlobalConfigInstance.VrrpInstances.Instances[index].LeaseID)); err != nil {
		return nil
	}
	return nil
}

func (e *EtcdClient) unregister(ctx context.Context, index int) error {
	delCtx, cancel := context.WithTimeout(ctx, time.Duration(e.ttl)*time.Second)
	defer cancel()
	if _, err := e.cli.Revoke(delCtx, config.GlobalConfigInstance.VrrpInstances.Instances[index].LeaseID); err != nil {
		log.Printf("revoke lease err: %v", err)
	}
	key, _ := config.GlobalConfigInstance.VrrpInstances.Instances[index].GenerateKV()
	if _, err := e.cli.Delete(delCtx, key); err != nil {
		log.Printf("delete key: %s failed: %v", key, err)
		return err
	}
	config.GlobalConfigInstance.VrrpInstances.Instances[index].KeepAliveCh = nil
	return nil
}

func (e *EtcdClient) startKeepalive(ctx context.Context) {
	var wg sync.WaitGroup
	for i, ins := range config.GlobalConfigInstance.VrrpInstances.Instances {
		k, v := ins.GenerateKV()
		txn := e.cli.Txn(ctx)
		// 若key不存在则创建
		txn.If(clientv3.Compare(clientv3.CreateRevision(k), "=", 0)).Then(clientv3.OpPut(k, v)).Else()
		// 提交事务
		if _, err := txn.Commit(); err != nil {
			log.Printf("transcation commit failed: %v", err)
			// TODO bug-7488 写KV失败不应退出，应在goroutine中定时尝试写
			// continue
		}
		wg.Add(1)
		go func(index int, k, v string) {
			defer wg.Done()
			timer := time.NewTimer(time.Duration(e.ttl) * time.Second)
			defer timer.Stop()
			for {
				select {
				case <-util.NotifyDown.Ch:
					// 通知取消key续约，并删除key
					log.Printf("notify cancel key lease[k:%s, v:%s]", k, v)
					if err := e.unregister(ctx, index); err != nil {
						log.Printf("unregister[k:%s, v:%s] failed:%v", k, v, err)
						config.GlobalConfigInstance.VrrpInstances.Instances[index].HaveResidualInfo = true
					}
					return
				case res := <-config.GlobalConfigInstance.VrrpInstances.Instances[index].KeepAliveCh:
					// 如果res为空，代表保活通道关闭
					if res == nil {
						log.Printf("try to register[k:%s, v:%s]", k, v)
						if err := e.register(ctx, index); err != nil {
							log.Printf("register[k:%s, v:%s] failed: %v", k, v, err)
							config.GlobalConfigInstance.VrrpInstances.Instances[index].HaveResidualInfo = true
						}
					}
				case <-timer.C:
					// 定时检查保活状态，keepaliveCh为nil表示保活通道关闭
					timer.Reset(time.Duration(e.ttl) * time.Second)
					// 清理残余租约信息
					if config.GlobalConfigInstance.VrrpInstances.Instances[index].HaveResidualInfo {
						if err := e.unregister(ctx, index); err != nil {
							log.Printf("unregister[k:%s, v:%s] failed: %v", k, v, err)
						} else {
							log.Printf("unregister[k:%s, v:%s] successful", k, v)
							config.GlobalConfigInstance.VrrpInstances.Instances[index].HaveResidualInfo = false
						}
					}
					// 如果保活通道关闭，重新注册
					if config.GlobalConfigInstance.VrrpInstances.Instances[index].KeepAliveCh == nil && !config.GlobalConfigInstance.VrrpInstances.Instances[index].HaveResidualInfo {
						log.Printf("try to register[k:%s, v:%s]", k, v)
						if err := e.register(ctx, index); err != nil {
							log.Printf("register[k:%s, v:%s] failed: %v", k, v, err)
							config.GlobalConfigInstance.VrrpInstances.Instances[index].HaveResidualInfo = true
						}
					}
				}
			}
		}(i, k, v)
	}
	wg.Wait()
}
