package util

import "sync"

var NotifyDown = &Channel{
	Ch: make(chan any),
}

type Channel struct {
	sync.Mutex
	Closed bool
	Ch     chan any
}
