package main

import (
	"sync"
	"time"
)

type Throttle struct {
	sleep    time.Duration
	increase float64
	mu       sync.RWMutex
}

func NewThrottle(sleep int64, increase float64) *Throttle {
	return &Throttle{
		sleep:    time.Duration(sleep) * time.Millisecond,
		increase: increase,
	}
}

func (t *Throttle) Sleep() time.Duration {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.sleep
}

func (t *Throttle) Throttle() {
	t.mu.Lock()
	defer t.mu.Unlock()

	newSleep := float64(t.sleep) * t.increase
	t.sleep = time.Duration(newSleep)
}
