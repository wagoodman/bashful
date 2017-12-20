package main

import (
	"bytes"
	"fmt"
	"html/template"
	"math/rand"
	"os"
	"os/signal"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	color "github.com/mgutz/ansi"
	"github.com/urfave/cli"
	terminal "github.com/wayneashleyberry/terminal-dimensions"
)

const (
	VERSION      = "v0.0.5"
	MAJOR_FORMAT = "cyan+b"
	INFO_FORMAT  = "blue+b"
)

var (
	exitSignaled     = false
	startTime        = time.Now()
	totalTasks       = 0
	completedTasks   = 0
	totalFailedTasks = 0

	purple                      = color.ColorFunc("magenta+h")
	red                         = color.ColorFunc("red+h")
	blue                        = color.ColorFunc("blue+h")
	boldblue                    = color.ColorFunc("blue+b")
	boldcyan                    = color.ColorFunc("cyan+b")
	bold                        = color.ColorFunc("default+b")
	lineDefaultTemplate, _      = template.New("default line").Parse(` {{.Status}}  ` + color.Reset + ` {{printf "%1s" .Spinner}} {{printf "%-25s" .Title}} {{.Msg}}{{.Split}}{{.Eta}}`)
	lineParallelTemplate, _     = template.New("parallel line").Parse(` {{.Status}}  ` + color.Reset + ` {{printf "%1s" .Spinner}} ├─ {{printf "%-25s" .Title}} {{.Msg}}{{.Split}}{{.Eta}}`)
	lineLastParallelTemplate, _ = template.New("last parallel line").Parse(` {{.Status}}  ` + color.Reset + ` {{printf "%1s" .Spinner}} ╰─ {{printf "%-25s" .Title}} {{.Msg}}{{.Split}}{{.Eta}}`)
	summaryTemplate, _          = template.New("summary line").Parse(` {{.Status}}    ` + color.Reset + ` {{printf "%-16s" .Percent}}` + color.Reset + ` {{.Steps}}{{.Errors}}{{.Msg}}{{.Split}}{{.Runtime}}{{.Eta}}`)
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

func footer(status CommandStatus) string {
	var tpl bytes.Buffer
	var durString, etaString, stepString, errorString string

	if config.Options.ShowSummaryTimes {
		duration := time.Since(startTime)
		durString = fmt.Sprintf(" Runtime[%s]", showDuration(duration))

		totalEta := time.Duration(config.totalEtaSeconds) * time.Second
		remainingEta := time.Duration(totalEta.Seconds()-duration.Seconds()) * time.Second
		etaString = fmt.Sprintf(" ETA[%s]", showDuration(remainingEta))
	}

	if completedTasks == totalTasks {
		etaString = ""
	}

	if config.Options.ShowStepSummary {
		stepString = fmt.Sprintf(" Tasks[%d/%d]", completedTasks, totalTasks)
	}

	if config.Options.ShowErrorSummary {
		errorString = fmt.Sprintf(" Errors[%d]", totalFailedTasks)
	}

	// get a string with the summary line without a split gap (eta floats left)
	percentValue := (float64(completedTasks) * float64(100)) / float64(totalTasks)
	percentStr := fmt.Sprintf("%3.2f%% Complete", percentValue)

	if completedTasks == totalTasks {
		percentStr = status.Color("b") + percentStr + color.Reset
	} else {
		percentStr = color.Color(percentStr, "default+b")
	}

	summaryTemplate.Execute(&tpl, Summary{Status: status.Color("i"), Percent: percentStr, Runtime: durString, Eta: etaString, Steps: stepString, Errors: errorString})

	// calculate a space buffer to push the eta to the right
	terminalWidth, _ := terminal.Width()
	splitWidth := int(terminalWidth) - visualLength(tpl.String())
	if splitWidth < 0 {
		splitWidth = 0
	}

	tpl.Reset()
	summaryTemplate.Execute(&tpl, Summary{Status: status.Color("i"), Percent: percentStr, Runtime: bold(durString), Eta: bold(etaString), Split: strings.Repeat(" ", splitWidth), Steps: bold(stepString), Errors: bold(errorString)})

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

func run(userYamlPath string) {
	var err error
	ReadConfig(userYamlPath)

	rand.Seed(time.Now().UnixNano())

	if config.Options.LogPath != "" {
		setupLogging()
	}

	if config.Options.UpdateInterval > 150 {
		ticker = time.NewTicker(time.Duration(config.Options.UpdateInterval) * time.Millisecond)
	} else {
		ticker = time.NewTicker(150 * time.Millisecond)
	}

	if config.Options.Vintage {
		config.Options.MaxParallelCmds = 1
		config.Options.ShowSummaryFooter = false
		config.Options.ShowFailureReport = false
		ticker.Stop()
	}

	var failedTasks []*Task

	fmt.Print("\033[?25l") // hide cursor
	logToMain("Running "+userYamlPath, MAJOR_FORMAT)
	for index := range config.Tasks {
		newFailedTasks := config.Tasks[index].RunAndDisplay()
		failedTasks = append(failedTasks, newFailedTasks...)

		if exitSignaled {
			break
		}
	}
	logToMain("Finished "+userYamlPath, MAJOR_FORMAT)

	err = Save(config.etaCachePath, &config.commandTimeCache)
	CheckError(err, "Unable to save command eta cache.")

	if config.Options.ShowSummaryFooter {
		Screen().ResetFrame(0, false, true)
		if len(failedTasks) > 0 {
			Screen().DisplayFooter(footer(StatusError))
		} else {
			Screen().DisplayFooter(footer(StatusSuccess))
		}
	}

	if len(failedTasks) > 0 {
		var buffer bytes.Buffer
		buffer.WriteString(red(" ...Some tasks failed, see below for details.\n"))

		for _, task := range failedTasks {

			buffer.WriteString("\n")
			buffer.WriteString(bold(red("⏺ Failed task: ")) + bold(task.Name) + "\n")
			buffer.WriteString(red("  ├─ command: ") + task.CmdString + "\n")
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
	fmt.Println(red(msg))
	os.Exit(1)
}

func exit(rc int) {
	cleanup()
	os.Exit(rc)
}

func cleanup() {
	// stop any running tasks
	for _, task := range config.Tasks {
		task.Kill()
	}

	// move the cursor past the used screen realestate
	Screen().MovePastFrame(true)

	// show the cursor again
	fmt.Print("\033[?25h") // show cursor
}

func main() {
	app := cli.NewApp()
	app.Name = "bashful"
	app.Version = VERSION
	app.Usage = "Takes a yaml file containing commands and bash snippits and executes each command while showing a simple (vertical) progress bar."
	app.Action = func(cliCtx *cli.Context) error {
		if cliCtx.NArg() < 1 {
			exitWithErrorMessage("Must provide the path to a bashful yaml file")
		} else if cliCtx.NArg() > 1 {
			exitWithErrorMessage("Only one bashful yaml file can be provided at a time")
		}
		userYamlPath := cliCtx.Args().Get(0)

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

		run(userYamlPath)
		return nil
	}

	app.Run(os.Args)

}
