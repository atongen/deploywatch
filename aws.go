package main

import (
	"errors"
	"math"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/codedeploy"
	"github.com/aws/aws-sdk-go/service/ec2"
)

// Aws interface hides all the difficult-to-manage string pointers
// that the api returns
type Aws interface {
	ListDeployments(string, string, []string) ([]string, error)
	GetDeployment(string) (*codedeploy.DeploymentInfo, error)
	ListDeploymentInstances(string) ([]string, error)
	DescribeInstances([]string) ([]*ec2.Instance, error)
	BatchGetDeploymentInstances(string, []string) ([]*codedeploy.InstanceSummary, error)
}

type awsEnv struct {
	sess   *session.Session
	cdSvc  *codedeploy.CodeDeploy
	ec2Svc *ec2.EC2
}

func NewAwsEnv() Aws {
	var a awsEnv = awsEnv{}

	// https://github.com/aws/aws-sdk-go/issues/384
	var opts session.Options = session.Options{
		SharedConfigState: session.SharedConfigEnable,
	}

	region := os.Getenv("AWS_REGION")
	if region != "" {
		opts.Config.Region = aws.String(region)
	}

	profile := os.Getenv("AWS_PROFILE")
	if profile != "" {
		opts.Profile = profile
	}

	// Create a session to share configuration, and load external configuration.
	a.sess = session.Must(session.NewSessionWithOptions(opts))
	a.cdSvc = codedeploy.New(a.sess)
	a.ec2Svc = ec2.New(a.sess)
	return &a
}

func (a *awsEnv) ListDeployments(applicationName, deploymentGroupName string, includeOnlyStatuses []string) ([]string, error) {
	input := &codedeploy.ListDeploymentsInput{}
	if applicationName != "" {
		input.SetApplicationName(applicationName)
	}
	if deploymentGroupName != "" {
		input.SetDeploymentGroupName(deploymentGroupName)
	}
	if len(includeOnlyStatuses) > 0 {
		input.SetIncludeOnlyStatuses(aws.StringSlice(includeOnlyStatuses))
	}

	var (
		deployments []string
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

		deployments = append(deployments, aws.StringValueSlice(resp.Deployments)...)

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

func (a *awsEnv) ListDeploymentInstances(deployId string) ([]string, error) {
	input := &codedeploy.ListDeploymentInstancesInput{}
	input.SetDeploymentId(deployId)

	var (
		instanceList []string
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

		instanceList = append(instanceList, aws.StringValueSlice(resp.InstancesList)...)

		if nextToken == nil {
			break
		}
	}

	return instanceList, nil
}

func (a *awsEnv) DescribeInstances(instanceIds []string) ([]*ec2.Instance, error) {
	var instances []*ec2.Instance

	// We can only ask for a maximum of 200 instance descriptions at a time
	partitionedInstanceIds := partition(instanceIds, 200)

	for _, ids := range partitionedInstanceIds {
		input := &ec2.DescribeInstancesInput{
			Filters: []*ec2.Filter{
				{
					Name:   aws.String("instance-id"),
					Values: aws.StringSlice(ids),
				},
			},
		}

		var (
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
	}

	return instances, nil
}

func (a *awsEnv) BatchGetDeploymentInstances(deployId string, instanceIds []string) ([]*codedeploy.InstanceSummary, error) {
	input := &codedeploy.BatchGetDeploymentInstancesInput{}
	input.SetDeploymentId(deployId)
	input.SetInstanceIds(aws.StringSlice(instanceIds))
	output, err := a.cdSvc.BatchGetDeploymentInstances(input)
	if err != nil {
		return nil, err
	}
	errMsg := strings.TrimSpace(aws.StringValue(output.ErrorMessage))
	if errMsg != "" {
		err = errors.New(errMsg)
		return nil, err
	}
	return output.InstancesSummary, nil
}

// partition splits a slice of strings into multiple
// sub-slices, each no longer than `size`
func partition(data []string, size int) [][]string {
	if size <= 0 {
		return [][]string{}
	}

	// we will end up with n slices of length size or less
	n := int(math.Ceil(float64(len(data)) / float64(size)))

	if n <= 0 {
		return [][]string{}
	}

	result := make([][]string, n)

	for i := 0; i < n; i++ {
		start := i * size
		end := min(len(data), (i+1)*size)
		subLen := end - start

		subslice := make([]string, subLen)

		for j := 0; j < subLen; j++ {
			subslice[j] = data[start+j]
		}

		result[i] = subslice
	}

	return result
}

func min(x, y int) int {
	return int(math.Min(float64(x), float64(y)))
}
