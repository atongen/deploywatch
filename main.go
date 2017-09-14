package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path"
	"strings"

	"github.com/gizak/termui"
)

// build flags
var (
	Version   string = "development"
	BuildTime string = "unset"
	BuildHash string = "unset"
	GoVersion string = "unset"
)

// cli flags
var (
	nameFlag        = flag.String("name", "", "CodeDeploy application name (optional)")
	groupsFlag      = flag.String("groups", "", "CodeDeploy deployment groups csv (optional)")
	compactFlag     = flag.Bool("compact", false, "Print compact output")
	hideSuccessFlag = flag.Bool("hide-success", false, "Do not print instances once they are successfully deployed")
	versionFlag     = flag.Bool("version", false, "Print version information and exit")
)

func versionInfo() string {
	return fmt.Sprintf("%s %s %s %s %s", path.Base(os.Args[0]), Version, BuildTime, BuildHash, GoVersion)
}

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "%s\n\nUsage: λ %s [OPTIONS] DEPLOY_ID [DEPLOY_ID]...\nOptions:\n", versionInfo(), os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()

	if *versionFlag {
		fmt.Fprintf(os.Stderr, "%s\n", versionInfo())
		os.Exit(0)
	}

	logFile, err := os.OpenFile("/tmp/deploywatch.log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		fmt.Printf("error opening log file: %v", err)
		os.Exit(1)
	}
	defer logFile.Close()

	logger := log.New(logFile, "", log.Lshortfile)

	aws := NewAwsEnv()
	renderer := NewRenderer(*compactFlag, *hideSuccessFlag)
	checker := NewChecker()

	quitCh := make(chan bool)
	renderCh := make(chan []byte)

	err = termui.Init()
	if err != nil {
		logger.Printf("Error creating terminal: %s\n", err)
		os.Exit(1)
	}
	defer termui.Close()

	par := termui.NewPar("")
	par.BorderLabel = "AWS CodeDeploy (press any key to quit)"
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

	termui.Handle("/sys/kbd", func(termui.Event) {
		quitCh <- true
	})

	// periodically check for updated deployment information
	// Created | Queued | InProgress | Succeeded | Failed | Stopped | Ready
	iosCreated := "Created"
	iosQueued := "Queued"
	iosInProgress := "InProgress"
	includeOnlyStatuses := []*string{&iosCreated, &iosQueued, &iosInProgress}

	checkDeploymentIds := NewSet()
	for _, deploymentId := range flag.Args() {
		checkDeploymentIds.Add(deploymentId)
	}

	groups := strings.Split(*groupsFlag, ",")
	checker.Check(5, func() {
		for _, group := range groups {
			if group != "" {
				currentDeployments, err := aws.ListDeployments(*nameFlag, group, includeOnlyStatuses)
				if err != nil {
					logger.Printf("Error getting deployments: %s %s %s\n", *nameFlag, group, err)
				} else {
					for _, deploymentIdPtr := range currentDeployments {
						checkDeploymentIds.Add(*deploymentIdPtr)
					}
				}
			}
		}

		for _, deploymentId := range checkDeploymentIds.List() {
			_, err := renderer.AddDeployment(aws, deploymentId)
			if err != nil {
				logger.Printf("Error getting deployment information: %s\n", err)
			}
		}
	})

	t := NewThrottle(1000, 1.05)

	// re-render current content every second
	checker.Check(1, func() {
		renderCh <- renderer.Bytes()
	})

	checkInstanceIds := NewSet()
	// periodically check renderer for new instances
	checker.Check(2, func() {
		for _, deploymentId := range renderer.DeploymentIds() {
			for _, instanceId := range renderer.InstanceIds(deploymentId) {
				if !checkInstanceIds.Has(instanceId) {
					checkInstanceIds.Add(instanceId)
					// begin checking instance
					checker.CheckInstance(1, deploymentId, instanceId, func(dId, iId string) {
						if !renderer.IsInstanceDone(iId) {
							summary, err := aws.GetDeploymentInstance(dId, iId)
							if err != nil {
								t.Throttle()
								logger.Printf("Error getting deployment instance summary (%s/%s): %s\n", dId, iId, err)
								logger.Printf("Instance check throttle set to %f\n", t.GetSleep())
							} else {
								renderCh <- renderer.Update(summary)
							}

							t.Sleep()
						}
					})
				}
			}
		}
	})

	// start goroutine aggregating rendered content
	checker.Updater(renderCh, func(content []byte) {
		termui.SendCustomEvt("/usr/t", content)
	})

	// listen for quit signal
	checker.Quiter(quitCh, func() {
		termui.StopLoop()
	})

	termui.Loop()

	close(quitCh)
	close(renderCh)
}
