package main

import (
	"flag"
	"fmt"
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
	nameFlag    = flag.String("name", "", "CodeDeploy application name (optional)")
	groupsFlag  = flag.String("groups", "", "CodeDeploy deployment groups csv (optional)")
	compactFlag = flag.Bool("compact", false, "Print compact output")
	versionFlag = flag.Bool("version", false, "Print version information and exit")
	verboseFlag = flag.Bool("verbose", false, "Print verbose output")
)

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "λ %s [OPTIONS] DEPLOY_ID [DEPLOY_ID]...\nOptions:\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()

	if *versionFlag {
		fmt.Printf("%s %s %s %s %s\n", path.Base(os.Args[0]), Version, BuildTime, BuildHash, GoVersion)
		os.Exit(0)
	}

	aws := NewAwsEnv()
	renderer := NewRenderer(*compactFlag)
	checker := NewChecker()

	quitCh := make(chan bool)
	renderCh := make(chan []byte)

	err := termui.Init()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating terminal: %s\n", err)
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

	termui.Handle("/sys/kbd/q", func(termui.Event) {
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
			currentDeployments, err := aws.ListDeployments(*nameFlag, group, includeOnlyStatuses)
			if err != nil {
				if *verboseFlag {
					fmt.Printf("Error getting deployments: %s %s %s\n", *nameFlag, group, err)
				}
			} else {
				for _, deploymentIdPtr := range currentDeployments {
					checkDeploymentIds.Add(*deploymentIdPtr)
				}
			}
		}

		for _, deploymentId := range checkDeploymentIds.List() {
			_, err := renderer.AddDeployment(aws, deploymentId)
			if err != nil {
				if *verboseFlag {
					fmt.Printf("Error getting deployment information: %s\n", err)
				}
			}
		}
	})

	checkInstanceIds := NewSet()
	// periodically check renderer for new instances
	checker.Check(3, func() {
		for _, deploymentId := range renderer.DeploymentIds() {
			for _, instanceId := range renderer.InstanceIds(deploymentId) {
				if !checkInstanceIds.Has(instanceId) {
					checkInstanceIds.Add(instanceId)
					// begin checking instance
					checker.CheckInstance(2, deploymentId, instanceId, func(dId, iId string) {
						if !renderer.IsInstanceDone(iId) {
							summary, err := aws.GetDeploymentInstance(dId, iId)
							if err != nil {
								if *verboseFlag {
									fmt.Fprintf(os.Stderr, "Error getting deployment instance summary (%s/%s): %s\n", dId, iId, err)
								}
								return
							}

							renderCh <- renderer.Update(summary)
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
