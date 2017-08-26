package main

import (
	"bytes"
	"sort"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go/service/codedeploy"
	"github.com/aws/aws-sdk-go/service/ec2"
)

type Renderer struct {
	Deployments           []*codedeploy.DeploymentInfo
	DeploymentInstanceMap map[string]*Set
	Instances             map[string]*ec2.Instance
	InstanceSummaries     map[string]*codedeploy.InstanceSummary
	compact               bool
	hideSuccess           bool
	mu                    sync.RWMutex
}

func NewRenderer(compact, hideSuccess bool) *Renderer {
	return &Renderer{
		[]*codedeploy.DeploymentInfo{},
		map[string]*Set{},
		map[string]*ec2.Instance{},
		map[string]*codedeploy.InstanceSummary{},
		compact,
		hideSuccess,
		sync.RWMutex{},
	}
}

func (r *Renderer) GetDeployment(deploymentId string) *codedeploy.DeploymentInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for i := 0; i < len(r.Deployments); i++ {
		dId := *r.Deployments[i].DeploymentId
		if dId == deploymentId {
			return r.Deployments[i]
		}
	}

	return nil
}

func (r *Renderer) AddDeployment(aws Aws, deploymentId string) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// ensure we don't know about this deployment already
	for _, deployment := range r.Deployments {
		if *deployment.DeploymentId == deploymentId {
			return false, nil
		}
	}

	deployment, err := aws.GetDeployment(deploymentId)
	if err != nil {
		return false, err
	}

	// build deployment instance map
	instanceIds, err := aws.ListDeploymentInstances(deploymentId)
	if err != nil {
		return false, err
	}

	// get instance data
	ec2Instances, err := aws.DescribeInstances(instanceIds)
	if err != nil {
		return false, err
	}

	r.Deployments = append(r.Deployments, deployment)
	if _, ok := r.DeploymentInstanceMap[deploymentId]; !ok {
		r.DeploymentInstanceMap[deploymentId] = NewSet()
	}

	for _, ec2Instance := range ec2Instances {
		instanceId := *ec2Instance.InstanceId
		r.DeploymentInstanceMap[deploymentId].Add(instanceId)
		r.Instances[instanceId] = ec2Instance
	}

	return true, nil
}

func (r *Renderer) getBytes() []byte {
	var b bytes.Buffer

	for _, deployment := range r.Deployments {
		deploymentId := *deployment.DeploymentId
		instanceIds := r.DeploymentInstanceMap[deploymentId].List()

		// short-circuit for deployments with 0 instances
		if len(instanceIds) == 0 {
			continue
		}

		numSuccess := r.countSuccess(instanceIds)
		sort.Strings(instanceIds)

		b.WriteString(DeploymentLine(deployment, numSuccess, len(instanceIds)))

		for _, instanceId := range instanceIds {
			instance := r.Instances[instanceId]

			summary := r.InstanceSummaries[instanceId]
			status := *summary.Status
			if status == "Success" && r.hideSuccess {
				continue
			}

			if r.compact {
				b.WriteString(CompactInstanceLine(instance, summary, r.maxInstanceNameLength()))
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

func (r *Renderer) Bytes() []byte {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.getBytes()
}

func (r *Renderer) String() string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return string(r.getBytes())
}

func (r *Renderer) maxInstanceNameLength() int {
	max := 0
	for _, instance := range r.Instances {
		l := len([]rune(InstanceName(instance)))
		if l > max {
			max = l
		}
	}
	return max
}

func (r *Renderer) DeploymentIds() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	list := make([]string, 0)
	for item := range r.DeploymentInstanceMap {
		list = append(list, item)
	}
	return list
}

func (r *Renderer) InstanceIds(deploymentId string) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if instances, ok := r.DeploymentInstanceMap[deploymentId]; ok {
		return instances.List()
	}

	return []string{}
}

func (r *Renderer) countSuccess(instanceIds []string) int {
	total := 0
	for _, instanceId := range instanceIds {
		if summary, ok := r.InstanceSummaries[instanceId]; ok {
			status := *summary.Status
			if status == "Success" {
				total += 1
			}
		}
	}
	return total
}

func (r *Renderer) IsInstanceDone(instanceId string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if summary, ok := r.InstanceSummaries[instanceId]; ok {
		status := *summary.Status
		if status != "Pending" && status != "InProgress" {
			return true
		}
	}

	return false
}

func (r *Renderer) Update(summary *codedeploy.InstanceSummary) []byte {
	r.mu.Lock()
	defer r.mu.Unlock()

	instanceArnId := *summary.InstanceId
	result := strings.Split(instanceArnId, "/")
	if len(result) == 2 {
		r.InstanceSummaries[result[1]] = summary
	}

	return r.getBytes()
}
