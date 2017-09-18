package main

import (
	"sync"
	"time"
)

type Throttle struct {
	sleep        time.Duration
	minSleep     time.Duration
	delta        float64
	lastThrottle time.Time
	mu           sync.Mutex
}

func NewThrottle(sleep float64, delta float64) *Throttle {
	return &Throttle{
		sleep:        time.Duration(sleep) * time.Second,
		minSleep:     time.Duration(sleep) * time.Second,
		delta:        delta,
		lastThrottle: time.Now(),
	}
}

func (t *Throttle) Sleep() time.Duration {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.sleep > t.minSleep {
		dist := time.Since(t.lastThrottle)
		if dist*2 > t.sleep {
			newSleep := float64(t.sleep) * (1.0 - t.delta)
			if newSleep > float64(t.minSleep) {
				t.sleep = time.Duration(newSleep)
			} else {
				t.sleep = t.minSleep
			}
		}
	}

	return t.sleep
}

func (t *Throttle) Throttle() time.Duration {
	t.mu.Lock()
	defer t.mu.Unlock()

	newSleep := float64(t.sleep) * (1.0 + t.delta)
	t.sleep = time.Duration(newSleep)
	t.lastThrottle = time.Now()

	return t.sleep
}
