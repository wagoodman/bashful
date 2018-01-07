package main

import (
	"bytes"
	"fmt"
	"io"
	"math/rand"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"text/template"
	"time"

	color "github.com/mgutz/ansi"
	"github.com/mholt/archiver"
	"github.com/urfave/cli"
	terminal "github.com/wayneashleyberry/terminal-dimensions"
)

const (
	MAJOR_FORMAT = "cyan+b"
	INFO_FORMAT  = "blue+b"
	ERROR_FORMAT = "red+b"
)

var (
	Version            = "No version provided"
	GitCommit          = "No commit provided"
	BuildTime          = "No build timestamp provided"
	AllTasks           []*Task
	ticker             *time.Ticker
	exitSignaled       = false
	startTime          = time.Now()
	purple             = color.ColorFunc("magenta+h")
	red                = color.ColorFunc("red+h")
	blue               = color.ColorFunc("blue+h")
	boldblue           = color.ColorFunc("blue+b")
	boldcyan           = color.ColorFunc("cyan+b")
	bold               = color.ColorFunc("default+b")
	summaryTemplate, _ = template.New("summary line").Parse(` {{.Status}}    ` + color.Reset + ` {{printf "%-16s" .Percent}}` + color.Reset + ` {{.Steps}}{{.Errors}}{{.Msg}}{{.Split}}{{.Runtime}}{{.Eta}}`)
)

type Summary struct {
	Status  string
	Percent string
	Msg     string
	Runtime string
	Eta     string
	Split   string
	Steps   string
	Errors  string
}

func CheckError(err error, message string) {
	if err != nil {
		fmt.Println(red("Error:"))
		_, file, line, _ := runtime.Caller(1)
		fmt.Println("Line:", line, "\tFile:", file, "\n", err)
		exitWithErrorMessage(message)
	}
}

func showDuration(duration time.Duration) string {
	seconds := int64(duration.Seconds()) % 60
	minutes := int64(duration.Minutes()) % 60
	hours := int64(duration.Hours()) % 24
	return fmt.Sprintf("%02d:%02d:%02d", hours, minutes, seconds)
}

func footer(status CommandStatus, message string) string {
	var tpl bytes.Buffer
	var durString, etaString, stepString, errorString string

	if config.Options.ShowSummaryTimes {
		duration := time.Since(startTime)
		durString = fmt.Sprintf(" Runtime[%s]", showDuration(duration))

		totalEta := time.Duration(config.totalEtaSeconds) * time.Second
		remainingEta := time.Duration(totalEta.Seconds()-duration.Seconds()) * time.Second
		etaString = fmt.Sprintf(" ETA[%s]", showDuration(remainingEta))
	}

	if TaskStats.completedTasks == TaskStats.totalTasks {
		etaString = ""
	}

	if config.Options.ShowSummarySteps {
		stepString = fmt.Sprintf(" Tasks[%d/%d]", TaskStats.completedTasks, TaskStats.totalTasks)
	}

	if config.Options.ShowSummaryErrors {
		errorString = fmt.Sprintf(" Errors[%d]", TaskStats.totalFailedTasks)
	}

	// get a string with the summary line without a split gap (eta floats left)
	percentValue := (float64(TaskStats.completedTasks) * float64(100)) / float64(TaskStats.totalTasks)
	percentStr := fmt.Sprintf("%3.2f%% Complete", percentValue)

	if TaskStats.completedTasks == TaskStats.totalTasks {
		percentStr = status.Color("b") + percentStr + color.Reset
	} else {
		percentStr = color.Color(percentStr, "default+b")
	}

	summaryTemplate.Execute(&tpl, Summary{Status: status.Color("i"), Percent: percentStr, Runtime: durString, Eta: etaString, Steps: stepString, Errors: errorString, Msg: message})

	// calculate a space buffer to push the eta to the right
	terminalWidth, _ := terminal.Width()
	splitWidth := int(terminalWidth) - visualLength(tpl.String())
	if splitWidth < 0 {
		splitWidth = 0
	}

	tpl.Reset()
	summaryTemplate.Execute(&tpl, Summary{Status: status.Color("i"), Percent: percentStr, Runtime: bold(durString), Eta: bold(etaString), Split: strings.Repeat(" ", splitWidth), Steps: bold(stepString), Errors: bold(errorString), Msg: message})

	return tpl.String()
}

func doesFileExist(name string) bool {
	if _, err := os.Stat(name); err != nil {
		if os.IsNotExist(err) {
			return false
		}
	}
	return true
}

func bundle(userYamlPath, outputPath string) {
	archivePath := "bundle.tar.gz"

	ReadConfig(userYamlPath)
	AllTasks := CreateTasks()

	DownloadAssets(AllTasks)

	fmt.Println(bold("Bundling " + userYamlPath + " to " + outputPath))
	/*  */
	bashfulPath, err := filepath.Abs(os.Args[0])
	CheckError(err, "Could not find path to bashful")
	archiver.TarGz.Make(archivePath, []string{userYamlPath, bashfulPath, config.cachePath})

	execute := `#!/bin/bash
set -eu
export TMPDIR=$(mktemp -d /tmp/runner.XXXXXX)
ARCHIVE=$(awk '/^__BASHFUL_ARCHIVE__/ {print NR + 1; exit 0; }' $0)

tail -n+$ARCHIVE $0 | tar -xz -C $TMPDIR

pushd $TMPDIR > /dev/null
./bashful run {{.Runyaml}}
popd > /dev/null
rm -rf $TMPDIR 

exit 0

__BASHFUL_ARCHIVE__
`
	var buff bytes.Buffer
	var values = struct {
		Runyaml string
	}{
		Runyaml: filepath.Base(userYamlPath),
	}

	tmpl := template.New("test")
	tmpl, err = tmpl.Parse(execute)
	CheckError(err, "Failed to parse execute template")
	err = tmpl.Execute(&buff, values)
	CheckError(err, "Failed to render execute template")

	runnerPath := "./runner"
	runnerFh, err := os.Create(runnerPath)
	CheckError(err, "Unable to create runner executable file")
	defer runnerFh.Close()

	_, err = runnerFh.Write(buff.Bytes())
	CheckError(err, "Unable to write bootstrap script to runner executable file")

	archiveFh, err := os.Open(archivePath)
	CheckError(err, "Unable to open payload file")
	defer archiveFh.Close()
	defer os.Remove(archivePath)

	_, err = io.Copy(runnerFh, archiveFh)
	CheckError(err, "Unable to write payload to runner executable file")

	err = os.Chmod(runnerPath, 0755)
	CheckError(err, "Unable to change runner permissions")

}

func run(userYamlPath string) {
	var err error
	fmt.Print("\033[?25l") // hide cursor

	ReadConfig(userYamlPath)
	AllTasks := CreateTasks()

	DownloadAssets(AllTasks)

	rand.Seed(time.Now().UnixNano())

	if config.Options.UpdateInterval > 150 {
		ticker = time.NewTicker(time.Duration(config.Options.UpdateInterval) * time.Millisecond)
	} else {
		ticker = time.NewTicker(150 * time.Millisecond)
	}

	var failedTasks []*Task

	tagInfo := ""
	if len(config.Cli.RunTags) > 0 {
		if config.Cli.ExecuteOnlyMatchedTags {
			tagInfo = " with only matching tags: "
		} else {
			tagInfo = " with non-tagged and matching tags: "
		}
		tagInfo += strings.Join(config.Cli.RunTags, ", ")
	}

	fmt.Println(bold("Running " + userYamlPath + tagInfo))
	logToMain("Running "+userYamlPath+tagInfo, MAJOR_FORMAT)

	for _, task := range AllTasks {
		task.Run()
		failedTasks = append(failedTasks, task.failedTasks...)

		if exitSignaled {
			break
		}
	}
	logToMain("Finished "+userYamlPath, MAJOR_FORMAT)

	err = Save(config.etaCachePath, &config.commandTimeCache)
	CheckError(err, "Unable to save command eta cache.")

	if config.Options.ShowSummaryFooter {
		message := ""
		Screen().ResetFrame(0, false, true)
		if len(failedTasks) > 0 {
			if config.Options.LogPath != "" {
				message = bold(" See log for details (" + config.Options.LogPath + ")")
			}
			Screen().DisplayFooter(footer(StatusError, message))
		} else {
			Screen().DisplayFooter(footer(StatusSuccess, message))
		}
	}

	if len(failedTasks) > 0 {
		var buffer bytes.Buffer
		buffer.WriteString(red(" ...Some tasks failed, see below for details.\n"))

		for _, task := range failedTasks {

			buffer.WriteString("\n")
			buffer.WriteString(bold(red("⏺ Failed task: ")) + bold(task.Config.Name) + "\n")
			buffer.WriteString(red("  ├─ command: ") + task.Config.CmdString + "\n")
			buffer.WriteString(red("  ├─ return code: ") + strconv.Itoa(task.Command.ReturnCode) + "\n")
			buffer.WriteString(red("  ╰─ stderr: ") + task.ErrorBuffer.String() + "\n")

		}
		logToMain(buffer.String(), "")

		// we may not show the error report, but we always log it.
		if config.Options.ShowFailureReport {
			fmt.Print(buffer.String())
		}

	}

	logToMain("Exiting", "")

	cleanup()
}

func exitWithErrorMessage(msg string) {
	cleanup()
	fmt.Println(os.Stderr, red(msg))
	os.Exit(1)
}

func exit(rc int) {
	cleanup()
	os.Exit(rc)
}

func cleanup() {
	// stop any running tasks
	for _, task := range AllTasks {
		task.Kill()
	}

	// move the cursor past the used screen realestate
	Screen().MovePastFrame(true)

	// show the cursor again
	fmt.Print("\033[?25h") // show cursor
}

func setup() {
	sigChannel := make(chan os.Signal, 2)
	signal.Notify(sigChannel, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		for sig := range sigChannel {
			if sig == syscall.SIGINT {
				exitWithErrorMessage(red("Keyboard Interrupt"))
			} else if sig == syscall.SIGTERM {
				exit(0)
			} else {
				exitWithErrorMessage("Unknown Signal: " + sig.String())
			}
		}
	}()
}

func main() {
	// var bundlePath string
	//app := cli.NewApp()
	//app.Name = "bashful"
	//app.Version = VERSION
	//app.Usage = "Takes a yaml file containing commands and bash snipits and executes each command while showing a simple (vertical) progress bar."
	//app.Flags = []cli.Flag{
	//	cli.StringFlag{
	//		Name:  "bundle, b",
	//		Usage: "Bundle yaml and referenced resources into a single executable `FILE`",
	//		Destination: &bundlePath,
	//	},
	//}
	//app.Action = func(cliCtx *cli.Context) error {
	//	setup()
	//
	//	userYamlPath := cliCtx.Args().Get(0)
	//
	//	fmt.Println(bundlePath)
	//	if bundlePath != "" {
	//		bundle(userYamlPath, bundlePath)
	//	} else {
	//		//if cliCtx.NArg() < 1 {
	//		//	exitWithErrorMessage("Must provide the path to a bashful yaml file")
	//		//} else if cliCtx.NArg() > 1 {
	//		//	exitWithErrorMessage("Only one bashful yaml file can be provided at a time")
	//		//}
	//
	//		run(userYamlPath)
	//	}
	//	return nil
	//}

	setup()
	app := cli.NewApp()
	app.Name = "bashful"
	app.Version = Version + "\nCommit: " + GitCommit + "\nBuild timestamp: " + BuildTime
	app.Usage = "Takes a yaml file containing commands and bash snippits and executes each command while showing a simple (vertical) progress bar."
	app.Commands = []cli.Command{
		{
			Name:  "bundle",
			Usage: "Bundle yaml and referenced resources into a single executable",
			Action: func(cliCtx *cli.Context) error {
				if cliCtx.NArg() < 1 {
					exitWithErrorMessage("Must provide the path to a bashful yaml file")
				} else if cliCtx.NArg() > 1 {
					exitWithErrorMessage("Only one bashful yaml file can be provided at a time")
				}

				userYamlPath := cliCtx.Args().Get(0)
				bundlePath := userYamlPath + ".bundle"

				bundle(userYamlPath, bundlePath)

				return nil
			},
		},
		{
			Name:  "run",
			Usage: "Execute the given yaml file with bashful",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "tags",
					Value: "",
					Usage: "A comma delimited list of matching task tags. If a task's tag matches *or if it is not tagged* then it will be executed (also see --only-tags).",
				},
				cli.StringFlag{
					Name:  "only-tags",
					Value: "",
					Usage: "A comma delimited list of matching task tags. A task will only be executed if it has a matching tag.",
				},
			},
			Action: func(cliCtx *cli.Context) error {
				if cliCtx.NArg() < 1 {
					exitWithErrorMessage("Must provide the path to a bashful yaml file")
				} else if cliCtx.NArg() > 1 {
					exitWithErrorMessage("Only one bashful yaml file can be provided at a time")
				}

				userYamlPath := cliCtx.Args().Get(0)

				if cliCtx.String("tags") != "" && cliCtx.String("only-tags") != "" {
					exitWithErrorMessage("Options 'tags' and 'only-tags' are mutually exclusive.")
				}

				for _, value := range strings.Split(cliCtx.String("tags"), ",") {
					if value != "" {
						config.Cli.RunTags = append(config.Cli.RunTags, value)
					}
				}

				for _, value := range strings.Split(cliCtx.String("only-tags"), ",") {
					if value != "" {
						config.Cli.ExecuteOnlyMatchedTags = true
						config.Cli.RunTags = append(config.Cli.RunTags, value)
					}
				}
				//config.Cli.RunTags = []string{"some-app1"}

				run(userYamlPath)

				return nil
			},
		},
	}

	app.Run(os.Args)

}
