package core

import (
	"text/template"
	"time"
	"strings"
	"fmt"
	"bytes"
	"strconv"
	"math/rand"
	"os/exec"
	"github.com/howeyc/gopass"
	color "github.com/mgutz/ansi"
	terminal "github.com/wayneashleyberry/terminal-dimensions"
)

const (
	majorFormat = "cyan+b"
	infoFormat  = "blue+b"
	errorFormat = "red+b"
)

var (
	allTasks           []*Task
	ticker             *time.Ticker
	exitSignaled       bool
	startTime          time.Time
	sudoPassword       string
	purple             = color.ColorFunc("magenta+h")
	red                = color.ColorFunc("red+h")
	blue               = color.ColorFunc("blue+h")
	bold               = color.ColorFunc("default+b")
	summaryTemplate, _ = template.New("summary line").Parse(` {{.Status}}    ` + color.Reset + ` {{printf "%-16s" .Percent}}` + color.Reset + ` {{.Steps}}{{.Errors}}{{.Msg}}{{.Split}}{{.Runtime}}{{.Eta}}`)
)

type summary struct {
	Status  string
	Percent string
	Msg     string
	Runtime string
	Eta     string
	Split   string
	Steps   string
	Errors  string
}


func Run(yamlString []byte, environment map[string]string) []*Task {
	var err error

	exitSignaled = false
	startTime = time.Now()

	ParseConfig(yamlString)
	allTasks = CreateTasks()
	storeSudoPasswd()

	DownloadAssets(allTasks)

	rand.Seed(time.Now().UnixNano())

	if Config.Options.UpdateInterval > 150 {
		ticker = time.NewTicker(time.Duration(Config.Options.UpdateInterval) * time.Millisecond)
	} else {
		ticker = time.NewTicker(150 * time.Millisecond)
	}

	var failedTasks []*Task

	tagInfo := ""
	if len(Config.Cli.RunTags) > 0 {
		if Config.Cli.ExecuteOnlyMatchedTags {
			tagInfo = " only matching tags: "
		} else {
			tagInfo = " non-tagged and matching tags: "
		}
		tagInfo += strings.Join(Config.Cli.RunTags, ", ")
	}

	fmt.Println(bold("Running " + tagInfo))
	LogToMain("Running "+tagInfo, majorFormat)

	for _, task := range allTasks {
		task.Run(environment)
		failedTasks = append(failedTasks, task.failedTasks...)

		if exitSignaled {
			break
		}
	}
	LogToMain("Complete", majorFormat)

	err = Save(Config.etaCachePath, &Config.commandTimeCache)
	CheckError(err, "Unable to save command eta cache.")

	if Config.Options.ShowSummaryFooter {
		message := ""
		newScreen().ResetFrame(0, false, true)
		if len(failedTasks) > 0 {
			if Config.Options.LogPath != "" {
				message = bold(" See log for details (" + Config.Options.LogPath + ")")
			}
			newScreen().DisplayFooter(footer(statusError, message))
		} else {
			newScreen().DisplayFooter(footer(statusSuccess, message))
		}
	}

	if len(failedTasks) > 0 {
		var buffer bytes.Buffer
		buffer.WriteString(red(" ...Some tasks failed, see below for details.\n"))

		for _, task := range failedTasks {

			buffer.WriteString("\n")
			buffer.WriteString(bold(red("• Failed task: ")) + bold(task.Config.Name) + "\n")
			buffer.WriteString(red("  ├─ command: ") + task.Config.CmdString + "\n")
			buffer.WriteString(red("  ├─ return code: ") + strconv.Itoa(task.Command.ReturnCode) + "\n")
			buffer.WriteString(red("  └─ stderr: ") + task.ErrorBuffer.String() + "\n")

		}
		LogToMain(buffer.String(), "")

		// we may not show the error report, but we always log it.
		if Config.Options.ShowFailureReport {
			fmt.Print(buffer.String())
		}

	}

	return failedTasks
}


func storeSudoPasswd() {
	var sout bytes.Buffer

	// check if there is a task that requires sudo
	requireSudo := false
	for _, task := range allTasks {
		if task.Config.Sudo {
			requireSudo = true
			break
		}
		for _, subTask := range task.Children {
			if subTask.Config.Sudo {
				requireSudo = true
				break
			}
		}
	}

	if !requireSudo {
		return
	}

	// test if a password is even required for sudo
	cmd := exec.Command("/bin/sh", "-c", "sudo -Sn /bin/true")
	cmd.Stderr = &sout
	err := cmd.Run()
	requiresPassword := sout.String() == "sudo: a password is required\n"

	if requiresPassword {
		fmt.Print("[bashful] sudo password required: ")
		sudoPassword, err := gopass.GetPasswd()
		CheckError(err, "Could get sudo password from user.")

		// test the given password
		cmdTest := exec.Command("/bin/sh", "-c", "sudo -S /bin/true")
		cmdTest.Stdin = strings.NewReader(string(sudoPassword) + "\n")
		err = cmdTest.Run()
		if err != nil {
			ExitWithErrorMessage("Given sudo password did not work.")
		}
	} else {
		CheckError(err, "Could not determine sudo access for user.")
	}
}



func footer(status CommandStatus, message string) string {
	var tpl bytes.Buffer
	var durString, etaString, stepString, errorString string

	if Config.Options.ShowSummaryTimes {
		duration := time.Since(startTime)
		durString = fmt.Sprintf(" Runtime[%s]", showDuration(duration))

		totalEta := time.Duration(Config.totalEtaSeconds) * time.Second
		remainingEta := time.Duration(totalEta.Seconds()-duration.Seconds()) * time.Second
		etaString = fmt.Sprintf(" ETA[%s]", showDuration(remainingEta))
	}

	if TaskStats.completedTasks == TaskStats.totalTasks {
		etaString = ""
	}

	if Config.Options.ShowSummarySteps {
		stepString = fmt.Sprintf(" Tasks[%d/%d]", TaskStats.completedTasks, TaskStats.totalTasks)
	}

	if Config.Options.ShowSummaryErrors {
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

	summaryTemplate.Execute(&tpl, summary{Status: status.Color("i"), Percent: percentStr, Runtime: durString, Eta: etaString, Steps: stepString, Errors: errorString, Msg: message})

	// calculate a space buffer to push the eta to the right
	terminalWidth, _ := terminal.Width()
	splitWidth := int(terminalWidth) - visualLength(tpl.String())
	if splitWidth < 0 {
		splitWidth = 0
	}

	tpl.Reset()
	summaryTemplate.Execute(&tpl, summary{Status: status.Color("i"), Percent: percentStr, Runtime: bold(durString), Eta: bold(etaString), Split: strings.Repeat(" ", splitWidth), Steps: bold(stepString), Errors: bold(errorString), Msg: message})

	return tpl.String()
}