package main

import (
	"sync"
	"time"
)

type Throttle struct {
	sleep        time.Duration
	delta        float64
	lastThrottle time.Time
	mu           sync.Mutex
}

func NewThrottle(sleep int64, delta float64) *Throttle {
	return &Throttle{
		sleep:        time.Duration(sleep) * time.Millisecond,
		delta:        delta,
		lastThrottle: time.Now(),
	}
}

func (t *Throttle) Sleep() time.Duration {
	t.mu.Lock()
	defer t.mu.Unlock()

	mySleep := t.sleep
	dist := time.Since(t.lastThrottle)
	if dist*2 > t.sleep {
		newSleep := float64(t.sleep) * (1.0 - t.delta)
		t.sleep = time.Duration(newSleep)
	}

	return mySleep
}

func (t *Throttle) Throttle() time.Duration {
	t.mu.Lock()
	defer t.mu.Unlock()

	newSleep := float64(t.sleep) * (1.0 + t.delta)
	t.sleep = time.Duration(newSleep)
	t.lastThrottle = time.Now()

	return t.sleep
}
