package server

import (
	"sync"
	"sync/atomic"
)

// Sub 发布订阅
type Sub struct {
	ch     chan any
	filter func(entry any) bool
}

func New() *PubSub {
	return &PubSub{}
}

type PubSub struct {
	subs           []*Sub
	numSubscribers int32
	sync.RWMutex
}

// Publish 发布，推送给所有订阅者
func (ps *PubSub) Publish(item interface{}) {
	ps.RLock()
	defer ps.RUnlock()

	for _, sub := range ps.subs {
		if sub.filter == nil || sub.filter(item) {
			select {
			case sub.ch <- item:
			default:
			}
		}
	}
}

// Subscribe 订阅
func (ps *PubSub) Subscribe(subCh chan interface{}, doneCh <-chan struct{}, filter func(entry interface{}) bool) {
	ps.Lock()
	defer ps.Unlock()

	sub := &Sub{subCh, filter}
	ps.subs = append(ps.subs, sub)
	atomic.AddInt32(&ps.numSubscribers, 1)
	go func() {
		<-doneCh
		ps.Lock()
		defer ps.Unlock()
		for i, s := range ps.subs {
			if s == sub {
				ps.subs = append(ps.subs[:i], ps.subs[i+1:]...)
				break // 切片修改后，需要立即退出循环，否则会导致切片越界panic
			}
		}
		atomic.AddInt32(&ps.numSubscribers, -1)
	}()
}

// NumSubscribers 当前订阅者数量
func (ps *PubSub) NumSubscribers() int32 {
	return atomic.LoadInt32(&ps.numSubscribers)
}
