package main

import (
	"bytes"
	"time"
)

type Checker struct {
	quiters []chan bool
}

func NewChecker() *Checker {
	return &Checker{
		[]chan bool{},
	}
}

func (c *Checker) Quit() {
	for i := len(c.quiters) - 1; i >= 0; i-- {
		c.quiters[i] <- true
		close(c.quiters[i])
	}
}

func (c *Checker) Quiter(quitCh <-chan bool, fn func()) {
	go func(qCh <-chan bool) {
		<-qCh
		c.Quit()
		fn()
	}(quitCh)
}

func (c *Checker) Check(seconds int, fn func()) {
	q := make(chan bool)
	go func() {
		// call the function prior to ticker timeout
		fn()
		ticker := time.NewTicker(time.Duration(seconds) * time.Second)
		for {
			select {
			case <-ticker.C:
				fn()
			case <-q:
				return
			}
		}
	}()
	c.quiters = append(c.quiters, q)
}

type CheckInstanceFunc func(string, string)

func (c *Checker) CheckInstance(seconds int, deploymentId, instanceId string, fn CheckInstanceFunc) {
	q := make(chan bool)
	go func(myDeploymentId, myInstanceId string) {
		// call the function prior to ticker timeout
		fn(myDeploymentId, myInstanceId)
		ticker := time.NewTicker(time.Duration(seconds) * time.Second)
		for {
			select {
			case <-ticker.C:
				fn(myDeploymentId, myInstanceId)
			case <-q:
				return
			}
		}
	}(deploymentId, instanceId)
	c.quiters = append(c.quiters, q)
}

func (c *Checker) Updater(dataCh <-chan []byte, fn func([]byte)) {
	q := make(chan bool)
	go func(dataCh <-chan []byte) {
		currentData := make([]byte, 0)
		for {
			select {
			case data := <-dataCh:
				if !bytes.Equal(data, currentData) {
					currentData = data
					fn(currentData)
				}
			case <-q:
				return
			}
		}
	}(dataCh)
	c.quiters = append(c.quiters, q)
}
