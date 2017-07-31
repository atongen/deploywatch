package main

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/codedeploy"
	"github.com/aws/aws-sdk-go/service/ec2"
)

type Aws interface {
	ListDeployments(string, string, []*string) ([]*string, error)
	GetDeployment(string) (*codedeploy.DeploymentInfo, error)
	ListDeploymentInstances(string) ([]*string, error)
	DescribeInstances([]*string) ([]*ec2.Instance, error)
	GetDeploymentInstance(string, string) (*codedeploy.InstanceSummary, error)
}

type awsEnv struct {
	sess   *session.Session
	cdSvc  *codedeploy.CodeDeploy
	ec2Svc *ec2.EC2
}

func NewAwsEnv() Aws {
	var a awsEnv = awsEnv{}
	// Create a session to share configuration, and load external configuration.
	a.sess = session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	}))
	a.cdSvc = codedeploy.New(a.sess)
	a.ec2Svc = ec2.New(a.sess)
	return &a
}

func (a *awsEnv) ListDeployments(applicationName, deploymentGroupName string, includeOnlyStatuses []*string) ([]*string, error) {
	input := &codedeploy.ListDeploymentsInput{}
	if applicationName != "" {
		input.SetApplicationName(applicationName)
	}
	if deploymentGroupName != "" {
		input.SetDeploymentGroupName(deploymentGroupName)
	}
	if len(includeOnlyStatuses) > 0 {
		input.SetIncludeOnlyStatuses(includeOnlyStatuses)
	}

	var (
		deployments []*string
		nextToken   *string
	)

	for {
		if nextToken != nil {
			input.NextToken = nextToken
		}

		resp, err := a.cdSvc.ListDeployments(input)
		if err != nil {
			return nil, err
		}

		nextToken = resp.NextToken

		deployments = append(deployments, resp.Deployments...)

		if nextToken == nil {
			break
		}
	}

	return deployments, nil
}

func (a *awsEnv) GetDeployment(deployId string) (*codedeploy.DeploymentInfo, error) {
	input := &codedeploy.GetDeploymentInput{}
	input.SetDeploymentId(deployId)
	output, err := a.cdSvc.GetDeployment(input)
	if err != nil {
		return nil, err
	}

	return output.DeploymentInfo, nil
}

func (a *awsEnv) ListDeploymentInstances(deployId string) ([]*string, error) {
	input := &codedeploy.ListDeploymentInstancesInput{}
	input.SetDeploymentId(deployId)

	var (
		instanceList []*string
		nextToken    *string
	)

	for {
		if nextToken != nil {
			input.NextToken = nextToken
		}

		resp, err := a.cdSvc.ListDeploymentInstances(input)
		if err != nil {
			return nil, err
		}

		nextToken = resp.NextToken

		instanceList = append(instanceList, resp.InstancesList...)

		if nextToken == nil {
			break
		}
	}

	return instanceList, nil
}

func (a *awsEnv) DescribeInstances(instanceIds []*string) ([]*ec2.Instance, error) {
	input := &ec2.DescribeInstancesInput{
		Filters: []*ec2.Filter{
			{
				Name:   aws.String("instance-id"),
				Values: instanceIds,
			},
		},
	}

	var (
		instances []*ec2.Instance
		nextToken *string
	)

	for {
		if nextToken != nil {
			input.NextToken = nextToken
		}

		resp, err := a.ec2Svc.DescribeInstances(input)
		if err != nil {
			return nil, err
		}

		nextToken = resp.NextToken

		for _, res := range resp.Reservations {
			instances = append(instances, res.Instances...)
		}

		if nextToken == nil {
			break
		}
	}

	return instances, nil
}

func (a *awsEnv) GetDeploymentInstance(deployId, instanceId string) (*codedeploy.InstanceSummary, error) {
	input := &codedeploy.GetDeploymentInstanceInput{}
	input.SetDeploymentId(deployId)
	input.SetInstanceId(instanceId)
	summary, err := a.cdSvc.GetDeploymentInstance(input)
	if err != nil {
		return nil, err
	}

	return summary.InstanceSummary, nil
}
