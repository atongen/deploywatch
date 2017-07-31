package main

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/codedeploy"
	"github.com/aws/aws-sdk-go/service/ec2"
)

func GetAwsSession() *session.Session {
	// Create a session to share configuration, and load external configuration.
	return session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	}))
}

func GetCodeDeployService(sess *session.Session) *codedeploy.CodeDeploy {
	return codedeploy.New(sess)
}

func GetEc2Service(sess *session.Session) *ec2.EC2 {
	return ec2.New(sess)
}

func ListDeployments(svc *codedeploy.CodeDeploy, applicationName, deploymentGroupName string, includeOnlyStatuses []*string) ([]*string, error) {
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

		resp, err := svc.ListDeployments(input)
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

func GetDeployment(svc *codedeploy.CodeDeploy, deployId string) (*codedeploy.DeploymentInfo, error) {
	input := &codedeploy.GetDeploymentInput{}
	input.SetDeploymentId(deployId)
	output, err := svc.GetDeployment(input)
	if err != nil {
		return nil, err
	}

	return output.DeploymentInfo, nil
}

func ListDeploymentInstances(svc *codedeploy.CodeDeploy, deployId string) ([]*string, error) {
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

		resp, err := svc.ListDeploymentInstances(input)
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

func DescribeInstances(svc *ec2.EC2, instanceIds []*string) ([]*ec2.Instance, error) {
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

		resp, err := svc.DescribeInstances(input)
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

func GetDeploymentInstance(svc *codedeploy.CodeDeploy, deployId, instanceId string) (*codedeploy.InstanceSummary, error) {
	input := &codedeploy.GetDeploymentInstanceInput{}
	input.SetDeploymentId(deployId)
	input.SetInstanceId(instanceId)
	summary, err := svc.GetDeploymentInstance(input)
	if err != nil {
		return nil, err
	}

	return summary.InstanceSummary, nil
}
