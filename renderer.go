package main

import (
	"bytes"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go/service/codedeploy"
	"github.com/aws/aws-sdk-go/service/ec2"
)

type Renderer struct {
	Deployments           []*codedeploy.DeploymentInfo
	DeploymentInstanceMap map[string][]string
	Instances             map[string]*ec2.Instance
	InstanceSummaries     map[string]*codedeploy.InstanceSummary
	maxInstNameLen        int
	compact               bool
	mu                    sync.Mutex
}

func NewRenderer(compact bool) *Renderer {
	return &Renderer{
		[]*codedeploy.DeploymentInfo{},
		map[string][]string{},
		map[string]*ec2.Instance{},
		map[string]*codedeploy.InstanceSummary{},
		-1,
		compact,
		sync.Mutex{},
	}
}

func (r *Renderer) Bytes() []byte {
	var b bytes.Buffer

	for _, deployment := range r.Deployments {
		deploymentId := *deployment.DeploymentId
		instanceIds := []string{}

		for _, instanceId := range r.DeploymentInstanceMap[deploymentId] {
			if instanceId == "" {
				continue
			}

			if _, ok := r.Instances[instanceId]; !ok {
				continue
			}

			instanceIds = append(instanceIds, instanceId)
		}

		// short-circuit for deployments with 0 instances
		if len(instanceIds) == 0 {
			continue
		}

		b.WriteString(DeploymentLine(deployment))

		for _, instanceId := range instanceIds {
			instance := r.Instances[instanceId]

			summary := r.InstanceSummaries[instanceId]

			if r.compact {
				b.WriteString(CompactInstanceLine(instance, summary, r.MaxInstanceNameLength()))
			} else {
				b.WriteString(InstanceLine(instance))

				if summary != nil {
					for _, lifecycleEvent := range summary.LifecycleEvents {
						b.WriteString(LifecycleEventLine(lifecycleEvent))
					}
				}
			}
		}
	}

	return b.Bytes()
}

func (r *Renderer) String() string {
	return string(r.Bytes())
}

func (r *Renderer) MaxInstanceNameLength() int {
	if r.maxInstNameLen < 0 {
		max := 0
		for _, instance := range r.Instances {
			l := len([]rune(InstanceName(instance)))
			if l > max {
				max = l
			}
		}
		r.maxInstNameLen = max
	}
	return r.maxInstNameLen
}

func (r *Renderer) Update(summary *codedeploy.InstanceSummary) []byte {
	r.mu.Lock()
	defer r.mu.Unlock()

	instanceArnId := *summary.InstanceId
	result := strings.Split(instanceArnId, "/")
	if len(result) == 2 {
		r.InstanceSummaries[result[1]] = summary
	}

	return r.Bytes()
}
