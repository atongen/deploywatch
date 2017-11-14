package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path"
	"strings"
	"time"

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
	logFileFlag     = flag.String("log-file", "/tmp/deploywatch.log", "Location of log file")
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

	logFile, err := os.OpenFile(*logFileFlag, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		fmt.Printf("error opening log file: %v", err)
		os.Exit(1)
	}
	defer logFile.Close()

	logger := log.New(logFile, "", log.LstdFlags|log.Lshortfile)

	aws := NewAwsEnv()
	renderer := NewRenderer(*compactFlag, *hideSuccessFlag)
	checker := NewChecker(logger)

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
			if !renderer.HasDeployment(deploymentId) {
				logger.Printf("Starting to check deployment %s\n", deploymentId)
			}
			err := renderer.AddDeployment(aws, deploymentId)
			if err != nil {
				logger.Printf("Error getting deployment information: %s\n", err)
			}
		}
	})

	checkInstances := map[string]*Set{}
	doneInstances := NewSet()

	// periodically update list of instances to check
	checker.Check(1, func() {
		for _, deploymentId := range renderer.DeploymentIds() {
			if _, ok := checkInstances[deploymentId]; !ok {
				checkInstances[deploymentId] = NewSet()
			}

			for _, instanceId := range renderer.InstanceIds(deploymentId) {
				if !checkInstances[deploymentId].Has(instanceId) {
					logger.Printf("Starting to check instance %s (%s)\n", instanceId, deploymentId)
					checkInstances[deploymentId].Add(instanceId)
				}

				if renderer.IsInstanceDone(instanceId) && !doneInstances.Has(instanceId) {
					logger.Printf("Done checking instance %s (%s)\n", instanceId, deploymentId)
					doneInstances.Add(instanceId)
				}
			}
		}
	})

	t := NewThrottle(5.0, 0.025)

	checker.Check(10, func() {
		for deploymentId, instanceIds := range checkInstances {
			batchCheckInstances := instanceIds.Dif(doneInstances).List()

			if len(batchCheckInstances) == 0 {
				continue
			}

			summaries, err := aws.BatchGetDeploymentInstances(deploymentId, batchCheckInstances)
			if err != nil {
				sleep := t.Throttle()
				logger.Printf("Error getting deployment instance summaries %s: %s\n", deploymentId, err)
				logger.Printf("Instance check throttle increased to %s\n", sleep)
				time.Sleep(sleep)
			} else {
				// touch throttle for sleep decay
				_ = t.Sleep()
			}

			renderCh <- renderer.BatchUpdate(summaries)
		}
	})

	// start goroutine aggregating rendered content
	checker.Updater(renderCh, func(content []byte) {
		termui.SendCustomEvt("/usr/t", content)
	})

	// listen for quit signal
	checker.Quiter(quitCh, func() {
		logger.Printf("Goodbye!")
		termui.StopLoop()
	})

	termui.Loop()

	close(quitCh)
	close(renderCh)
}
