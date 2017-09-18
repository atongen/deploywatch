package main

import (
	"bytes"
	"log"
	"time"
)

type Checker struct {
	quiters []chan bool
	logger  *log.Logger
}

func NewChecker(logger *log.Logger) *Checker {
	return &Checker{
		[]chan bool{},
		logger,
	}
}

func (c *Checker) Quit() {
	c.logger.Println("Starting to quit")
	for i := len(c.quiters) - 1; i >= 0; i-- {
		go func(qCh chan<- bool) {
			qCh <- true
			close(qCh)
		}(c.quiters[i])
	}
}

func (c *Checker) Quiter(quitCh <-chan bool, fn func()) {
	go func(qCh <-chan bool) {
		<-qCh
		c.logger.Println("Received quit signal")
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
