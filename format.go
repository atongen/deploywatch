package main

import (
	"fmt"
	"math"

	"github.com/aws/aws-sdk-go/service/codedeploy"
	"github.com/aws/aws-sdk-go/service/ec2"
)

func DeploymentLine(deployment *codedeploy.DeploymentInfo) string {
	deployId := StrColor(*deployment.DeploymentId, "cyan")
	return fmt.Sprintf("%s %s-%s\n", deployId, *deployment.ApplicationName, *deployment.DeploymentGroupName)
}

func InstanceName(instance *ec2.Instance) string {
	for _, tag := range instance.Tags {
		if *tag.Key == "Name" {
			return *tag.Value
		}
	}

	return ""
}

func InstanceLine(instance *ec2.Instance) string {
	return fmt.Sprintf("  %s (%s)\n", StrColor(InstanceName(instance), "magenta"), *instance.InstanceId)
}

func CompactInstanceLine(instance *ec2.Instance, summary *codedeploy.InstanceSummary, maxLen int) string {
	name := InstanceName(instance)
	id := *instance.InstanceId
	var status string
	if summary != nil {
		status = StatusStr(*summary.Status)
	} else {
		status = StatusStr("Pending")
	}
	return fmt.Sprintf("  %s (%s) %s\n", PadRight(name, " ", maxLen), id, status)
}

func StatusStr(status string) string {
	var color string
	switch status {
	default:
	case "Pending":
		color = "yellow"
	case "InProgress":
		color = "blue"
	case "Succeeded":
		color = "green"
	case "Failed":
		color = "red"
	case "Skipped":
		color = "yellow"
	}
	return StrColor(status, color)
}

func StrColor(str, color string) string {
	return fmt.Sprintf("[%s](fg-%s)", str, color)
}

func SummaryLine(summary *codedeploy.InstanceSummary) string {
	status := StatusStr(*summary.Status)
	return fmt.Sprintf("    %s\n", status)
}

func LifecycleEventDuration(lifecycleEvent *codedeploy.LifecycleEvent) string {
	var duration int
	if lifecycleEvent.StartTime == nil || lifecycleEvent.StartTime.IsZero() {
		duration = 0
	} else {
		duration = int(math.Floor(lifecycleEvent.EndTime.Sub(*lifecycleEvent.StartTime).Seconds()))
	}
	return fmt.Sprintf("%4ds", duration)
}

func LifecycleEventName(name string) string {
	return fmt.Sprintf("%-20s", name)
}

func LifecycleEventLine(lifecycleEvent *codedeploy.LifecycleEvent) string {
	name := LifecycleEventName(*lifecycleEvent.LifecycleEventName)
	duration := LifecycleEventDuration(lifecycleEvent)
	status := StatusStr(*lifecycleEvent.Status)
	return fmt.Sprintf("    => %s %s %s\n", name, duration, status)
}
