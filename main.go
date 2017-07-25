package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/service/codedeploy"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/gizak/termui"
)

func BuildRenderer(cdSvc *codedeploy.CodeDeploy, ec2Svc *ec2.EC2, deployments []string, compact bool) *Renderer {
	renderer := NewRenderer(compact)

	// get all deployment data
	for _, deploymentId := range deployments {
		deployment, err := GetDeployment(cdSvc, deploymentId)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting deployment id %s: %s", deploymentId, err)
			continue
		}

		renderer.Deployments = append(renderer.Deployments, deployment)
	}

	// build deployment instance map
	for _, deployment := range renderer.Deployments {
		var deploymentId string = *deployment.DeploymentId

		instanceList, err := ListDeploymentInstances(cdSvc, deploymentId)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting deployment instances for %s: %s", deploymentId, err)
			continue
		}

		renderer.DeploymentInstanceMap[deploymentId] = PointerSliceToStrings(instanceList)
	}

	// get instance data
	for _, instanceIds := range renderer.DeploymentInstanceMap {
		ec2Instances, err := DescribeInstances(ec2Svc, StringSliceToPointers(instanceIds))

		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting ec2 instance data %s: %s", instanceIds, err)
			continue
		}

		for _, ec2Instance := range ec2Instances {
			instanceId := *ec2Instance.InstanceId
			renderer.Instances[instanceId] = ec2Instance
		}
	}

	return renderer
}

// build flags
var (
	Version   string = "development"
	BuildTime string = "unset"
	BuildHash string = "unset"
	GoVersion string = "unset"
)

// cli flags
var (
	compactFlag = flag.Bool("compact", false, "Print compact output")
	versionFlag = flag.Bool("version", false, "Print version information and exit")
)

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "λ %s [OPTIONS] DEPLOY_ID [DEPLOY_ID]...\nOptions:\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()

	if *versionFlag {
		fmt.Printf("deploywatch %s %s %s %s\n", Version, BuildTime, BuildHash, GoVersion)
		os.Exit(0)
	}

	sess := GetAwsSession()
	cdSvc := GetCodeDeployService(sess)
	ec2Svc := GetEc2Service(sess)

	deploymentIds := flag.Args()
	if len(deploymentIds) == 0 {
		fmt.Fprintf(os.Stderr, "No deployment IDs found!")
		os.Exit(1)
	}
	renderer := BuildRenderer(cdSvc, ec2Svc, deploymentIds, *compactFlag)

	if len(renderer.Instances) == 0 {
		fmt.Fprintf(os.Stderr, "No instances found in any deployments!")
		os.Exit(1)
	}

	err := termui.Init()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating terminal: %s", err)
		os.Exit(1)
	}
	defer termui.Close()

	par := termui.NewPar("")
	par.BorderLabel = "AWS CodeDeploy (type 'q' to quit)"
	par.TextFgColor = termui.ColorWhite
	par.BorderFg = termui.ColorGreen

	termui.Body.AddRows(termui.NewRow(termui.NewCol(12, 0, par)))

	termui.Body.Align()
	termui.Render(par)

	termui.Handle(("/usr"), func(e termui.Event) {
		trimContent := strings.TrimSpace(string(e.Data.([]byte)))
		par.Text = trimContent
		par.Height = strings.Count(trimContent, "\n") + 3
		termui.Body.Align()
		termui.Render(par)
	})

	quitCh := make(chan bool)

	termui.Handle("/sys/kbd/q", func(termui.Event) {
		quitCh <- true
	})

	doneChs := []chan bool{}
	renderCh := make(chan []byte)

	// start goroutine checking deploy instance
	for deploymentId, instances := range renderer.DeploymentInstanceMap {
		for _, instanceId := range instances {
			if instanceId == "" {
				continue
			}

			myDoneCh := make(chan bool)
			go func(dId, iId string, rCh chan<- []byte, dCh <-chan bool) {
				summary, err := GetDeploymentInstance(cdSvc, dId, iId)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error getting deployment instance summary (%s/%s): %s\n", dId, iId, err)
				} else {
					rCh <- renderer.Update(summary)
				}

				var iAmDone bool = false
				ticker := time.NewTicker(2 * time.Second)
				for {
					select {
					case <-ticker.C:
						if !iAmDone {
							summary, err := GetDeploymentInstance(cdSvc, dId, iId)
							if err != nil {
								fmt.Fprintf(os.Stderr, "Error getting deployment instance summary (%s/%s): %s\n", dId, iId, err)
								continue
							}

							rCh <- renderer.Update(summary)

							status := *summary.Status
							if status != "Pending" && status != "InProgress" {
								iAmDone = true
							}
						}
					case <-dCh:
						return
					}
				}
			}(deploymentId, instanceId, renderCh, myDoneCh)
			doneChs = append(doneChs, myDoneCh)
		}
	}

	// start goroutine aggregating rendered content
	myDoneCh := make(chan bool)
	go func(contentCh <-chan []byte, doneCh <-chan bool) {
		currentContent := make([]byte, 0)
		for {
			select {
			case content := <-contentCh:
				if !bytes.Equal(content, currentContent) {
					currentContent = content
					termui.SendCustomEvt("/usr/t", currentContent)
				}
			case <-doneCh:
				return
			}
		}
	}(renderCh, myDoneCh)
	doneChs = append(doneChs, myDoneCh)

	// start goroutine listening for quit signal
	go func(qCh <-chan bool, dChs []chan bool) {
		<-qCh
		for _, ch := range dChs {
			ch <- true
		}
		termui.StopLoop()
	}(quitCh, doneChs)

	termui.Loop()

	close(quitCh)
	close(renderCh)
	for _, ch := range doneChs {
		close(ch)
	}
}
