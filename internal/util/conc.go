package util

import (
	"log"
	"os"
	"sync"
	"syscall"
)

type Channel struct {
	sync.Mutex
	Closed bool
	Ch     chan any
}

var NotifyDown = &Channel{
	Ch: make(chan any),
}

func (c *Channel) IsClosed() bool {
	c.Lock()
	defer c.Unlock()
	return c.Closed
}

func (c *Channel) Renew() {
	c.Lock()
	defer c.Unlock()
	c.Ch = make(chan any)
	c.Closed = false
}

// CheckProcessPid 检查进程存活状态
func CheckProcessPid(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		log.Printf("unable to find process %d", pid)
		return false
	}
	if err := process.Signal(syscall.Signal(0)); err != nil {
		return false
	}
	return true
}
