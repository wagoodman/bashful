package handler

import (
	"bytes"
	"fmt"
	"github.com/google/uuid"
	color "github.com/mgutz/ansi"
	"github.com/tj/go-spin"
	"github.com/wagoodman/bashful/config"
	"github.com/wagoodman/bashful/runtime"
	"github.com/wagoodman/bashful/utils"
	"github.com/wagoodman/jotframe"
	"github.com/wayneashleyberry/terminal-dimensions"
	"io"
	"strconv"
	"strings"
	"sync"
	"text/template"
	"time"
)

type VerticalUI struct {
	lock      sync.Mutex
	data      map[uuid.UUID]*display
	spinner   *spin.Spinner
	ticker    *time.Ticker
	startTime time.Time
	executor  *runtime.Executor
	frame     *jotframe.FixedFrame
}

// display represents all non-Config items that control how the task line should be printed to the screen
type display struct {
	Task *runtime.Task

	// Template is the single-line string template that should be used to display the TaskStatus of a single task
	Template *template.Template

	// Index is the row within a screen frame to print the task template
	Index int

	// Values holds all template values that represent the task TaskStatus
	Values lineInfo

	line *jotframe.Line
}

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

// lineInfo represents all template values that represent the task status
type lineInfo struct {
	// Status is the current pending/running/error/success status of the command
	Status string

	// Title is the display name to use for the task
	Title string

	// Msg may show any arbitrary string to the screen (such as stdout or stderr values)
	Msg string

	// Prefix is used to place the spinner or bullet characters before the title
	Prefix string

	// Eta is the displayed estimated time to completion based on the current time
	Eta string

	// Split can be used to "float" values to the right hand side of the screen when printing a single line
	Split string
}

var (
	summaryTemplate, _ = template.New("summary line").Parse(` {{.Status}}    ` + color.Reset + ` {{printf "%-16s" .Percent}}` + color.Reset + ` {{.Steps}}{{.Errors}}{{.Msg}}{{.Split}}{{.Runtime}}{{.Eta}}`)

	// lineDefaultTemplate is the string template used to display the TaskStatus values of a single task with no children
	lineDefaultTemplate, _ = template.New("default line").Parse(` {{.Status}}  ` + color.Reset + ` {{printf "%1s" .Prefix}} {{printf "%-25s" .Title}} {{.Msg}}{{.Split}}{{.Eta}}`)

	// lineParallelTemplate is the string template used to display the TaskStatus values of a task that is the child of another task
	lineParallelTemplate, _ = template.New("parallel line").Parse(` {{.Status}}  ` + color.Reset + ` {{printf "%1s" .Prefix}} ├─ {{printf "%-25s" .Title}} {{.Msg}}{{.Split}}{{.Eta}}`)

	// lineLastParallelTemplate is the string template used to display the TaskStatus values of a task that is the LAST child of another task
	lineLastParallelTemplate, _ = template.New("last parallel line").Parse(` {{.Status}}  ` + color.Reset + ` {{printf "%1s" .Prefix}} └─ {{printf "%-25s" .Title}} {{.Msg}}{{.Split}}{{.Eta}}`)
)

func NewVerticalUI(updateInterval time.Duration) *VerticalUI {

	handler := &VerticalUI{
		data:      make(map[uuid.UUID]*display, 0),
		spinner:   spin.New(),
		ticker:    time.NewTicker(updateInterval),
		startTime: time.Now(),
	}

	go handler.spinnerHandler()

	return handler
}

func (handler *VerticalUI) spinnerHandler() {

	for {
		select {

		case <-handler.ticker.C:
			handler.lock.Lock()

			handler.spinner.Next()
			for _, displayData := range handler.data {
				task := displayData.Task

				if task.Config.CmdString != "" {
					if !task.Command.Complete && task.Command.Started {
						displayData.Values.Prefix = handler.spinner.Current()
						displayData.Values.Eta = task.CurrentEta()
					}
					handler.displayTask(task)
				}

				for _, subTask := range task.Children {
					childDisplayData := handler.data[subTask.Id]
					if !subTask.Command.Complete && subTask.Command.Started {
						childDisplayData.Values.Prefix = handler.spinner.Current()
						childDisplayData.Values.Eta = subTask.CurrentEta()
					}
					handler.displayTask(subTask)
				}

				// update the summary line
				if config.Config.Options.ShowSummaryFooter {
					renderedFooter := handler.footer(runtime.StatusPending, "", task.Executor)
					io.WriteString(handler.frame.Footer(), renderedFooter)
				}
			}
			handler.lock.Unlock()
		}

	}
}

// todo: move footer logic based on jotframe requirements
func (handler *VerticalUI) Close() {
	// todo: remove config references
	if config.Config.Options.ShowSummaryFooter {
		// todo: add footer update via Executor stats
		message := ""

		handler.frame.Footer().Open()
		if len(handler.executor.FailedTasks) > 0 {
			if config.Config.Options.LogPath != "" {
				message = utils.Bold(" See log for details (" + config.Config.Options.LogPath + ")")
			}

			renderedFooter := handler.footer(runtime.StatusError, message, handler.executor)
			handler.frame.Footer().WriteStringAndClose(renderedFooter)
		} else {
			renderedFooter := handler.footer(runtime.StatusSuccess, message, handler.executor)
			handler.frame.Footer().WriteStringAndClose(renderedFooter)
		}
		handler.frame.Footer().Close()
	}
}

func (handler *VerticalUI) Unregister(task *runtime.Task) {
	if _, ok := handler.data[task.Id]; !ok {
		// ignore data that have already been unregistered
		return
	}
	handler.lock.Lock()
	defer handler.lock.Unlock()

	if len(task.Children) > 0 {

		displayData := handler.data[task.Id]

		hasHeader := len(task.Children) > 0
		collapseSection := task.Config.CollapseOnCompletion && hasHeader && task.FailedChildren == 0

		// complete the proc group TaskStatus
		if hasHeader {
			handler.frame.Header().Open()
			var message bytes.Buffer
			collapseSummary := ""
			if collapseSection {
				collapseSummary = utils.Purple(" (" + strconv.Itoa(len(task.Children)) + " Tasks hidden)")
			}
			displayData.Template.Execute(&message, lineInfo{Status: task.Status.Color("i"), Title: task.Config.Name + collapseSummary, Prefix: config.Config.Options.BulletChar})

			handler.frame.Header().WriteStringAndClose(message.String())
		}

		// collapse sections or parallel Tasks...
		if collapseSection {
			// todo: enhance jotframe to take care of this
			length := len(handler.frame.Lines())
			for idx := 0; idx < length; idx++ {
				handler.frame.Remove(handler.frame.Lines()[0])
			}
		}
	}
	handler.frame.Close()

	delete(handler.data, task.Id)
}

func (handler *VerticalUI) doRegister(task *runtime.Task) {
	if _, ok := handler.data[task.Id]; ok {
		// ignore data that have already been registered
		return
	}

	// todo: this is hackey
	if handler.executor == nil && task.Executor != nil {
		handler.executor = task.Executor
	}

	hasParentCmd := task.Config.CmdString != ""
	hasHeader := len(task.Children) > 0
	numTasks := len(task.Children)
	if hasParentCmd {
		numTasks++
	}

	// todo: remove isFirst logic
	isFirst := handler.frame == nil
	handler.frame = jotframe.NewFixedFrame(0, hasHeader, config.Config.Options.ShowSummaryFooter, false)
	if !isFirst {
		handler.frame.Move(-1)
	}

	var line *jotframe.Line
	if hasParentCmd {
		line, _ = handler.frame.Append()
		// todo: check err
	}

	handler.data[task.Id] = &display{
		Template: lineDefaultTemplate,
		Index:    0,
		Task:     task,
		line:     line,
	}
	for idx, subTask := range task.Children {
		line, _ := handler.frame.Append()
		// todo: check err
		handler.data[subTask.Id] = &display{
			Template: lineParallelTemplate,
			Index:    idx + 1,
			Task:     subTask,
			line:     line,
		}
		if idx == len(task.Children)-1 {
			handler.data[subTask.Id].Template = lineLastParallelTemplate
		}
	}

	displayData := handler.data[task.Id]

	// initialize each line in the frame
	if hasHeader {
		var message bytes.Buffer
		lineObj := lineInfo{Status: runtime.StatusRunning.Color("i"), Title: task.Config.Name, Msg: "", Prefix: config.Config.Options.BulletChar}
		displayData.Template.Execute(&message, lineObj)

		io.WriteString(handler.frame.Header(), message.String())
	}

	if hasParentCmd {
		displayData.Values = lineInfo{Status: runtime.StatusPending.Color("i"), Title: task.Config.Name}
		handler.displayTask(task)
	}

	for line := 0; line < len(task.Children); line++ {
		childDisplayData := handler.data[task.Children[line].Id]
		childDisplayData.Values = lineInfo{Status: runtime.StatusPending.Color("i"), Title: task.Children[line].Config.Name}
		handler.displayTask(task.Children[line])
	}
}

func (handler *VerticalUI) Register(task *runtime.Task) {
	handler.lock.Lock()
	defer handler.lock.Unlock()

	handler.doRegister(task)

}

func (handler *VerticalUI) OnEvent(task *runtime.Task, e runtime.TaskEvent) {
	handler.lock.Lock()
	defer handler.lock.Unlock()

	eventTask := e.Task

	if !eventTask.Config.ShowTaskOutput {
		e.Stderr = ""
		e.Stdout = ""
	}
	eventDisplayData := handler.data[e.Task.Id]

	if e.Stderr != "" {
		eventDisplayData.Values = lineInfo{
			Status: e.Status.Color("i"),
			Title:  eventTask.Config.Name,
			Msg:    e.Stderr,
			Prefix: handler.spinner.Current(),
			Eta:    eventTask.CurrentEta(),
		}
	} else {
		eventDisplayData.Values = lineInfo{
			Status: e.Status.Color("i"),
			Title:  eventTask.Config.Name,
			Msg:    e.Stdout,
			Prefix: handler.spinner.Current(),
			Eta:    eventTask.CurrentEta(),
		}
	}

	handler.displayTask(eventTask)

	// update the summary line
	if config.Config.Options.ShowSummaryFooter {
		renderedFooter := handler.footer(runtime.StatusPending, "", task.Executor)
		handler.frame.Footer().WriteString(renderedFooter)
	}
}

func (handler *VerticalUI) displayTask(task *runtime.Task) {

	// todo: error handling
	if _, ok := handler.data[task.Id]; !ok {
		return
	}

	terminalWidth, _ := terminaldimensions.Width()

	displayData := handler.data[task.Id]

	renderedLine := handler.renderTask(task, int(terminalWidth))
	io.WriteString(displayData.line, renderedLine)

}

func (handler *VerticalUI) footer(status runtime.TaskStatus, message string, executor *runtime.Executor) string {
	var tpl bytes.Buffer
	var durString, etaString, stepString, errorString string

	if config.Config.Options.ShowSummaryTimes {
		duration := time.Since(handler.startTime)
		durString = fmt.Sprintf(" Runtime[%s]", utils.ShowDuration(duration))

		totalEta := time.Duration(config.Config.TotalEtaSeconds) * time.Second
		remainingEta := time.Duration(totalEta.Seconds()-duration.Seconds()) * time.Second
		etaString = fmt.Sprintf(" ETA[%s]", utils.ShowDuration(remainingEta))
	}

	if len(executor.CompletedTasks) == executor.TotalTasks {
		etaString = ""
	}

	if config.Config.Options.ShowSummarySteps {
		stepString = fmt.Sprintf(" Tasks[%d/%d]", len(executor.CompletedTasks), executor.TotalTasks)
	}

	if config.Config.Options.ShowSummaryErrors {
		errorString = fmt.Sprintf(" Errors[%d]", len(executor.FailedTasks))
	}

	// get a string with the summary line without a split gap (eta floats left)
	percentValue := (float64(len(executor.CompletedTasks)) * float64(100)) / float64(executor.TotalTasks)
	percentStr := fmt.Sprintf("%3.2f%% Complete", percentValue)
	percentStr = color.Color(percentStr, "default+b")

	summaryTemplate.Execute(&tpl, summary{Status: status.Color("i"), Percent: percentStr, Runtime: durString, Eta: etaString, Steps: stepString, Errors: errorString, Msg: message})

	// calculate a space buffer to push the eta to the right
	terminalWidth, _ := terminaldimensions.Width()
	splitWidth := int(terminalWidth) - utils.VisualLength(tpl.String())
	if splitWidth < 0 {
		splitWidth = 0
	}

	tpl.Reset()
	summaryTemplate.Execute(&tpl, summary{Status: status.Color("i"), Percent: percentStr, Runtime: utils.Bold(durString), Eta: utils.Bold(etaString), Split: strings.Repeat(" ", splitWidth), Steps: utils.Bold(stepString), Errors: utils.Bold(errorString), Msg: message})

	return tpl.String()
}

// String represents the task status and command output in a single line
func (handler *VerticalUI) renderTask(task *runtime.Task, terminalWidth int) string {
	displayData := handler.data[task.Id]

	if task.Command.Complete {
		displayData.Values.Eta = ""
		if task.Command.ReturnCode != 0 && !task.Config.IgnoreFailure {
			displayData.Values.Msg = utils.Red("Exited with error (" + strconv.Itoa(task.Command.ReturnCode) + ")")
		}
	}

	// set the name
	if task.Config.Name == "" {
		if len(task.Config.CmdString) > 25 {
			task.Config.Name = task.Config.CmdString[:22] + "..."
		} else {
			task.Config.Name = task.Config.CmdString
		}
	}

	// display
	var message bytes.Buffer

	// get a string with the summary line without a split gap or message
	displayData.Values.Split = ""
	originalMessage := displayData.Values.Msg
	displayData.Values.Msg = ""
	displayData.Template.Execute(&message, displayData.Values)

	// calculate the max width of the message and trim it
	maxMessageWidth := terminalWidth - utils.VisualLength(message.String())
	displayData.Values.Msg = originalMessage
	if utils.VisualLength(displayData.Values.Msg) > maxMessageWidth {
		displayData.Values.Msg = utils.TrimToVisualLength(displayData.Values.Msg, maxMessageWidth-3) + "..."
	}

	// calculate a space buffer to push the eta to the right
	message.Reset()
	displayData.Template.Execute(&message, displayData.Values)
	splitWidth := terminalWidth - utils.VisualLength(message.String())
	if splitWidth < 0 {
		splitWidth = 0
	}

	message.Reset()

	// override the current spinner to empty or a config.Config.Options.BulletChar
	if (!task.Command.Started || task.Command.Complete) && len(task.Children) == 0 && displayData.Template == lineDefaultTemplate {
		displayData.Values.Prefix = config.Config.Options.BulletChar
	} else if task.Command.Complete {
		displayData.Values.Prefix = ""
	}

	displayData.Values.Split = strings.Repeat(" ", splitWidth)
	displayData.Template.Execute(&message, displayData.Values)

	return message.String()
}
