package runtime

import (
	"github.com/wagoodman/bashful/config"
	"strconv"
	"time"
	"fmt"
	"github.com/wagoodman/bashful/utils"

	"github.com/wayneashleyberry/terminal-dimensions"
	color "github.com/mgutz/ansi"
	"bytes"
)

type UIHandler struct {

}

func NewUIHandler() *UIHandler {
	return &UIHandler{

	}
}

func (handler *UIHandler) register(task *Task) {

	var message bytes.Buffer
	hasParentCmd := task.Config.CmdString != ""
	hasHeader := len(task.Children) > 0
	numTasks := len(task.Children)
	if hasParentCmd {
		numTasks++
	}
	scr := NewScreen()
	scr.ResetFrame(numTasks, hasHeader, config.Config.Options.ShowSummaryFooter)

	// make room for the title of a parallel proc group
	if hasHeader {
		message.Reset()
		lineObj := lineInfo{Status: statusRunning.Color("i"), Title: task.Config.Name, Msg: "", Prefix: config.Config.Options.BulletChar}
		task.Display.Template.Execute(&message, lineObj)
		scr.DisplayHeader(message.String())
	}

	if hasParentCmd {
		task.Display.Values = lineInfo{Status: statusPending.Color("i"), Title: task.Config.Name}
		handler.displayTask(task)
	}

	for line := 0; line < len(task.Children); line++ {
		task.Children[line].Display.Values = lineInfo{Status: statusPending.Color("i"), Title: task.Children[line].Config.Name}
		handler.displayTask(task.Children[line])
	}

}

func (handler *UIHandler) onEvent(task *Task, e event) {

	scr := NewScreen()
	eventTask := e.Task

	// update the state before displaying...
	if e.Complete {
		eventTask.Completed(e.ReturnCode)
		task.startAvailableTasks(task.Executor.environment)

		// todo: we shouldn't update non display values here, that should be done where the event occured
		task.status = e.Status

		if e.Status == StatusError {
			// todo: we shouldn't update non display values here, that should be done where the event occured
			// keep note of the failed task for an after task report
			task.FailedChildren++
			task.Executor.FailedTasks = append(task.Executor.FailedTasks, eventTask)
		}
	}

	if !eventTask.Config.ShowTaskOutput {
		e.Stderr = ""
		e.Stdout = ""
	}

	if e.Stderr != "" {
		eventTask.Display.Values = lineInfo{Status: e.Status.Color("i"), Title: eventTask.Config.Name, Msg: e.Stderr, Prefix: spinner.Current(), Eta: eventTask.CurrentEta()}
	} else {
		eventTask.Display.Values = lineInfo{Status: e.Status.Color("i"), Title: eventTask.Config.Name, Msg: e.Stdout, Prefix: spinner.Current(), Eta: eventTask.CurrentEta()}
	}

	handler.displayTask(eventTask)

	// update the summary line
	if config.Config.Options.ShowSummaryFooter {
		scr.DisplayFooter(footer(statusPending, "", task.Executor))
	} else {
		scr.MovePastFrame(false)
	}
}


func (handler *UIHandler) displayTask(task *Task) {
	terminalWidth, _ := terminaldimensions.Width()
	theScreen := NewScreen()
	if config.Config.Options.SingleLineDisplay {

		var durString, etaString, stepString, errorString string
		displayString := ""

		effectiveWidth := int(terminalWidth)

		fillColor := color.ColorCode(strconv.Itoa(config.Config.Options.ColorSuccess) + "+i")
		emptyColor := color.ColorCode(strconv.Itoa(config.Config.Options.ColorSuccess))
		if len(task.Executor.FailedTasks) > 0 {
			fillColor = color.ColorCode(strconv.Itoa(config.Config.Options.ColorError) + "+i")
			emptyColor = color.ColorCode(strconv.Itoa(config.Config.Options.ColorError))
		}

		numFill := int(effectiveWidth) * len(task.Executor.CompletedTasks) / task.Executor.TotalTasks

		if config.Config.Options.ShowSummaryTimes {
			duration := time.Since(startTime)
			durString = fmt.Sprintf(" Runtime[%s]", utils.ShowDuration(duration))

			totalEta := time.Duration(config.Config.TotalEtaSeconds) * time.Second
			remainingEta := time.Duration(totalEta.Seconds()-duration.Seconds()) * time.Second
			etaString = fmt.Sprintf(" ETA[%s]", utils.ShowDuration(remainingEta))
		}

		if len(task.Executor.CompletedTasks) == task.Executor.TotalTasks {
			etaString = ""
		}

		if config.Config.Options.ShowSummarySteps {
			stepString = fmt.Sprintf(" Tasks[%d/%d]", len(task.Executor.CompletedTasks), task.Executor.TotalTasks)
		}

		if config.Config.Options.ShowSummaryErrors {
			errorString = fmt.Sprintf(" Errors[%d]", len(task.Executor.FailedTasks))
		}

		valueStr := stepString + errorString + durString + etaString

		displayString = fmt.Sprintf("%[1]*s", -effectiveWidth, fmt.Sprintf("%[1]*s", (effectiveWidth+len(valueStr))/2, valueStr))
		displayString = fillColor + displayString[:numFill] + color.Reset + emptyColor + displayString[numFill:] + color.Reset

		theScreen.Display(displayString, 0)
	} else {
		theScreen.Display(task.String(int(terminalWidth)), task.Display.Index)
	}

}
