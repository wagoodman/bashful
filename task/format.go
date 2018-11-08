package task

import (
	"bytes"
	"time"
	"fmt"
	"github.com/wagoodman/bashful/utils"
	color "github.com/mgutz/ansi"
	"strings"
	"github.com/wagoodman/bashful/config"
	terminal "github.com/wayneashleyberry/terminal-dimensions"

	"strconv"
)

var (
	purple             = color.ColorFunc("magenta+h")
	red                = color.ColorFunc("red+h")
	blue               = color.ColorFunc("blue+h")
	bold               = color.ColorFunc("default+b")
)

// Color returns the ansi color value represented by the given CommandStatus
func (status CommandStatus) Color(attributes string) string {
	switch status {
	case statusRunning:
		return color.ColorCode(strconv.Itoa(config.Config.Options.ColorRunning) + "+" + attributes)

	case statusPending:
		return color.ColorCode(strconv.Itoa(config.Config.Options.ColorPending) + "+" + attributes)

	case StatusSuccess:
		return color.ColorCode(strconv.Itoa(config.Config.Options.ColorSuccess) + "+" + attributes)

	case StatusError:
		return color.ColorCode(strconv.Itoa(config.Config.Options.ColorError) + "+" + attributes)

	}
	return "INVALID COMMAND STATUS"
}


func footer(status CommandStatus, message string) string {
	var tpl bytes.Buffer
	var durString, etaString, stepString, errorString string

	if config.Config.Options.ShowSummaryTimes {
		duration := time.Since(StartTime)
		durString = fmt.Sprintf(" Runtime[%s]", utils.ShowDuration(duration))

		totalEta := time.Duration(config.Config.TotalEtaSeconds) * time.Second
		remainingEta := time.Duration(totalEta.Seconds()-duration.Seconds()) * time.Second
		etaString = fmt.Sprintf(" ETA[%s]", utils.ShowDuration(remainingEta))
	}

	if TaskStats.CompletedTasks == TaskStats.TotalTasks {
		etaString = ""
	}

	if config.Config.Options.ShowSummarySteps {
		stepString = fmt.Sprintf(" Tasks[%d/%d]", TaskStats.CompletedTasks, TaskStats.TotalTasks)
	}

	if config.Config.Options.ShowSummaryErrors {
		errorString = fmt.Sprintf(" Errors[%d]", TaskStats.TotalFailedTasks)
	}

	// get a string with the summary line without a split gap (eta floats left)
	percentValue := (float64(TaskStats.CompletedTasks) * float64(100)) / float64(TaskStats.TotalTasks)
	percentStr := fmt.Sprintf("%3.2f%% Complete", percentValue)

	if TaskStats.CompletedTasks == TaskStats.TotalTasks {
		percentStr = status.Color("b") + percentStr + color.Reset
	} else {
		percentStr = color.Color(percentStr, "default+b")
	}

	summaryTemplate.Execute(&tpl, summary{Status: status.Color("i"), Percent: percentStr, Runtime: durString, Eta: etaString, Steps: stepString, Errors: errorString, Msg: message})

	// calculate a space buffer to push the eta to the right
	terminalWidth, _ := terminal.Width()
	splitWidth := int(terminalWidth) - utils.VisualLength(tpl.String())
	if splitWidth < 0 {
		splitWidth = 0
	}

	tpl.Reset()
	summaryTemplate.Execute(&tpl, summary{Status: status.Color("i"), Percent: percentStr, Runtime: bold(durString), Eta: bold(etaString), Split: strings.Repeat(" ", splitWidth), Steps: bold(stepString), Errors: bold(errorString), Msg: message})

	return tpl.String()
}

// CurrentEta returns a formatted string indicating a countdown until command completion
func (task *Task) CurrentEta() string {
	var eta, etaValue string

	if config.Config.Options.ShowTaskEta {
		running := time.Since(task.Command.StartTime)
		etaValue = "Unknown!"
		if task.Command.EstimatedRuntime > 0 {
			etaValue = utils.ShowDuration(time.Duration(task.Command.EstimatedRuntime.Seconds()-running.Seconds()) * time.Second)
		}
		eta = fmt.Sprintf(bold("[%s]"), etaValue)
	}
	return eta
}
