package core

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"text/template"
	"time"

	"github.com/lunixbochs/vtclean"
	color "github.com/mgutz/ansi"
	"github.com/tj/go-spin"
	terminal "github.com/wayneashleyberry/terminal-dimensions"
	"github.com/wagoodman/bashful/config"
	"github.com/wagoodman/bashful/utils"
	"github.com/wagoodman/bashful/log"
)

const (
	majorFormat = "cyan+b"
	infoFormat  = "blue+b"
	errorFormat = "red+b"
)

var (
	// todo: none of these should be public
	SudoPassword       string
	ExitSignaled       bool
	StartTime          time.Time
	Ticker             *time.Ticker
	summaryTemplate, _ = template.New("summary line").Parse(` {{.Status}}    ` + color.Reset + ` {{printf "%-16s" .Percent}}` + color.Reset + ` {{.Steps}}{{.Errors}}{{.Msg}}{{.Split}}{{.Runtime}}{{.Eta}}`)
)


var (
	// spinner generates the spin icon character in front of running Tasks
	spinner = spin.New()

	// nextDisplayIdx is the next available screen row to use based off of the task / sub-task order.
	nextDisplayIdx = 0

	// lineDefaultTemplate is the string template used to display the status values of a single task with no children
	lineDefaultTemplate, _ = template.New("default line").Parse(` {{.Status}}  ` + color.Reset + ` {{printf "%1s" .Prefix}} {{printf "%-25s" .Title}} {{.Msg}}{{.Split}}{{.Eta}}`)

	// lineParallelTemplate is the string template used to display the status values of a task that is the child of another task
	lineParallelTemplate, _ = template.New("parallel line").Parse(` {{.Status}}  ` + color.Reset + ` {{printf "%1s" .Prefix}} ├─ {{printf "%-25s" .Title}} {{.Msg}}{{.Split}}{{.Eta}}`)

	// lineLastParallelTemplate is the string template used to display the status values of a task that is the LAST child of another task
	lineLastParallelTemplate, _ = template.New("last parallel line").Parse(` {{.Status}}  ` + color.Reset + ` {{printf "%1s" .Prefix}} └─ {{printf "%-25s" .Title}} {{.Msg}}{{.Split}}{{.Eta}}`)
)

const (
	statusRunning status = iota
	statusPending
	StatusSuccess
	StatusError
)

// NewTask creates a new task in the context of the user configuration at a particular screen location (row)
func NewTask(taskConfig config.TaskConfig, displayStartIdx int, replicaValue string) *Task {
	task := Task{Config: taskConfig}
	task.compile(displayStartIdx, replicaValue)

	for subIndex := range taskConfig.ParallelTasks {
		subTaskConfig := &taskConfig.ParallelTasks[subIndex]

		subTask := NewTask(*subTaskConfig, nextDisplayIdx, replicaValue)
		subTask.Display.Template = lineParallelTemplate
		task.Children = append(task.Children, subTask)
		nextDisplayIdx++
	}

	if len(task.Children) > 0 {
		task.Children[len(task.Children)-1].Display.Template = lineLastParallelTemplate
	}
	return &task
}

// compile is used by the constructor to finalize task runtime values
func (task *Task) compile(displayIdx int, replicaValue string) {
	task.compileCmd()

	task.Display.Template = lineDefaultTemplate
	task.Display.Index = displayIdx
	task.ErrorBuffer = bytes.NewBufferString("")

	task.resultChan = make(chan event)
	task.status = statusPending
}

func (task *Task) compileCmd() {
	if eta, ok := config.Config.CommandTimeCache[task.Config.CmdString]; ok {
		task.Command.EstimatedRuntime = eta
	} else {
		task.Command.EstimatedRuntime = time.Duration(-1)
	}

	shell := os.Getenv("SHELL")
	if len(shell) == 0 {
		shell = "sh"
	}

	readFd, writeFd, err := os.Pipe()
	utils.CheckError(err, "Could not open env pipe for child shell")

	sudoCmd := ""
	if task.Config.Sudo {
		sudoCmd = "sudo -S "
	}
	task.Command.Cmd = exec.Command(shell, "-c", sudoCmd+task.Config.CmdString+"; BASHFUL_RC=$?; env >&3; exit $BASHFUL_RC")
	task.Command.Cmd.Stdin = strings.NewReader(string(SudoPassword) + "\n")

	// Set current working directory; default is empty
	task.Command.Cmd.Dir = task.Config.CwdString

	// allow the child process to provide env vars via a pipe (FD3)
	task.Command.Cmd.ExtraFiles = []*os.File{writeFd}
	task.Command.EnvReadFile = readFd

	// set this command as a process group
	task.Command.Cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	task.Command.ReturnCode = -1
	task.Command.Environment = map[string]string{}
}

func (task *Task) UpdateExec(execpath string) {
	if task.Config.CmdString == "" {
		task.Config.CmdString = config.Config.Options.ExecReplaceString
	}
	task.Config.CmdString = strings.Replace(task.Config.CmdString, config.Config.Options.ExecReplaceString, execpath, -1)
	task.Config.URL = strings.Replace(task.Config.URL, config.Config.Options.ExecReplaceString, execpath, -1)

	task.compileCmd()
}

// Kill will stop any running command (including child Tasks) with a -9 signal
func (task *Task) Kill() {
	if task.Config.CmdString != "" && task.Command.Started && !task.Command.Complete {
		syscall.Kill(-task.Command.Cmd.Process.Pid, syscall.SIGKILL)
	}

	for _, subTask := range task.Children {
		if subTask.Config.CmdString != "" && subTask.Command.Started && !subTask.Command.Complete {
			syscall.Kill(-subTask.Command.Cmd.Process.Pid, syscall.SIGKILL)
		}
	}

}

// String represents the task status and command output in a single line
func (task *Task) String(terminalWidth int) string {

	if task.Command.Complete {
		task.Display.Values.Eta = ""
		if task.Command.ReturnCode != 0 && !task.Config.IgnoreFailure {
			task.Display.Values.Msg = red("Exited with error (" + strconv.Itoa(task.Command.ReturnCode) + ")")
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
	task.Display.Values.Split = ""
	originalMessage := task.Display.Values.Msg
	task.Display.Values.Msg = ""
	task.Display.Template.Execute(&message, task.Display.Values)

	// calculate the max width of the message and trim it
	maxMessageWidth := terminalWidth - utils.VisualLength(message.String())
	task.Display.Values.Msg = originalMessage
	if utils.VisualLength(task.Display.Values.Msg) > maxMessageWidth {
		task.Display.Values.Msg = utils.TrimToVisualLength(task.Display.Values.Msg, maxMessageWidth-3) + "..."
	}

	// calculate a space buffer to push the eta to the right
	message.Reset()
	task.Display.Template.Execute(&message, task.Display.Values)
	splitWidth := terminalWidth - utils.VisualLength(message.String())
	if splitWidth < 0 {
		splitWidth = 0
	}

	message.Reset()

	// override the current spinner to empty or a config.Config.Options.BulletChar
	if (!task.Command.Started || task.Command.Complete) && len(task.Children) == 0 && task.Display.Template == lineDefaultTemplate {
		task.Display.Values.Prefix = config.Config.Options.BulletChar
	} else if task.Command.Complete {
		task.Display.Values.Prefix = ""
	}

	task.Display.Values.Split = strings.Repeat(" ", splitWidth)
	task.Display.Template.Execute(&message, task.Display.Values)

	return message.String()
}

// display prints the current task string status to the screen
func (task *Task) display() {
	terminalWidth, _ := terminal.Width()
	theScreen := NewScreen()
	if config.Config.Options.SingleLineDisplay {

		var durString, etaString, stepString, errorString string
		displayString := ""

		effectiveWidth := int(terminalWidth)

		fillColor := color.ColorCode(strconv.Itoa(config.Config.Options.ColorSuccess) + "+i")
		emptyColor := color.ColorCode(strconv.Itoa(config.Config.Options.ColorSuccess))
		if len(task.invoker.FailedTasks) > 0 {
			fillColor = color.ColorCode(strconv.Itoa(config.Config.Options.ColorError) + "+i")
			emptyColor = color.ColorCode(strconv.Itoa(config.Config.Options.ColorError))
		}

		numFill := int(effectiveWidth) * len(task.invoker.CompletedTasks) / task.invoker.TotalTasks

		if config.Config.Options.ShowSummaryTimes {
			duration := time.Since(StartTime)
			durString = fmt.Sprintf(" Runtime[%s]", utils.ShowDuration(duration))

			totalEta := time.Duration(config.Config.TotalEtaSeconds) * time.Second
			remainingEta := time.Duration(totalEta.Seconds()-duration.Seconds()) * time.Second
			etaString = fmt.Sprintf(" ETA[%s]", utils.ShowDuration(remainingEta))
		}

		if len(task.invoker.CompletedTasks) == task.invoker.TotalTasks {
			etaString = ""
		}

		if config.Config.Options.ShowSummarySteps {
			stepString = fmt.Sprintf(" Tasks[%d/%d]", len(task.invoker.CompletedTasks), task.invoker.TotalTasks)
		}

		if config.Config.Options.ShowSummaryErrors {
			errorString = fmt.Sprintf(" Errors[%d]", len(task.invoker.FailedTasks))
		}

		valueStr := stepString + errorString + durString + etaString

		displayString = fmt.Sprintf("%[1]*s", -effectiveWidth, fmt.Sprintf("%[1]*s", (effectiveWidth+len(valueStr))/2, valueStr))
		displayString = fillColor + displayString[:numFill] + color.Reset + emptyColor + displayString[numFill:] + color.Reset

		theScreen.Display(displayString, 0)
	} else {
		theScreen.Display(task.String(int(terminalWidth)), task.Display.Index)
	}

}

func (task *Task) requiresSudoPasswd() bool {
	if task.Config.Sudo {
		return true
	}
	for _, subTask := range task.Children {
		if subTask.Config.Sudo {
			return true
		}
	}

	return false
}


// EstimateRuntime returns the ETA in seconds until command completion
func (task *Task) EstimateRuntime() float64 {
	var etaSeconds float64
	// finalize task by appending to the set of final Tasks
	if task.Config.CmdString != "" && task.Command.EstimatedRuntime != -1 {
		etaSeconds += task.Command.EstimatedRuntime.Seconds()
	}

	var maxParallelEstimatedRuntime float64
	var taskEndSecond []float64
	var currentSecond float64
	var remainingParallelTasks = config.Config.Options.MaxParallelCmds

	for subIndex := range task.Children {
		subTask := task.Children[subIndex]
		if subTask.Config.CmdString != "" && subTask.Command.EstimatedRuntime != -1 {
			// this is a sub task with an eta
			if remainingParallelTasks == 0 {

				// we've started all possible Tasks, now they should stop...
				// select the first task to stop
				remainingParallelTasks++
				minEndSecond, _, err := utils.MinMax(taskEndSecond)
				utils.CheckError(err, "No min eta for empty array!")
				taskEndSecond = utils.RemoveOneValue(taskEndSecond, minEndSecond)
				currentSecond = minEndSecond
			}

			// we are still starting Tasks
			taskEndSecond = append(taskEndSecond, currentSecond+subTask.Command.EstimatedRuntime.Seconds())
			remainingParallelTasks--

			_, maxEndSecond, err := utils.MinMax(taskEndSecond)
			utils.CheckError(err, "No max eta for empty array!")
			maxParallelEstimatedRuntime = math.Max(maxParallelEstimatedRuntime, maxEndSecond)
		}

	}
	etaSeconds += maxParallelEstimatedRuntime
	return etaSeconds
}

// variableSplitFunc splits a bytestream based on either newline characters or by length (if the string is too long)
func variableSplitFunc(data []byte, atEOF bool) (advance int, token []byte, err error) {

	// Return nothing if at end of file and no data passed
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}

	// Case: \n
	if i := strings.Index(string(data), "\n"); i >= 0 {
		return i + 1, data[0:i], nil
	}

	// Case: \r
	if i := strings.Index(string(data), "\r"); i >= 0 {
		return i + 1, data[0:i], nil
	}

	// Case: it's just too long
	terminalWidth, _ := terminal.Width()
	if len(data) > int(terminalWidth*2) {
		return int(terminalWidth * 2), data[0:int(terminalWidth*2)], nil
	}

	// TODO: by some ansi escape sequences

	// If at end of file with data return the data
	if atEOF {
		return len(data), data, nil
	}

	return
}

// runSingleCmd executes a Tasks primary command (not child task commands) and monitors command events
func (task *Task) runSingleCmd(owningResultChan chan event, owningWaiter *sync.WaitGroup, environment map[string]string) {
	log.LogToMain("Started Task: "+task.Config.Name, infoFormat)

	task.Command.StartTime = time.Now()

	owningResultChan <- event{Task: task, Status: statusRunning, ReturnCode: -1}
	owningWaiter.Add(1)
	defer owningWaiter.Done()

	tempFile, _ := ioutil.TempFile(config.Config.LogCachePath, "")
	task.LogFile = tempFile
	task.LogChan = make(chan log.LogItem)
	go log.SingleLogger(task.LogChan, task.Config.Name, tempFile.Name())

	stdoutPipe, _ := task.Command.Cmd.StdoutPipe()
	stderrPipe, _ := task.Command.Cmd.StderrPipe()

	// copy env vars into proc
	for k, v := range environment {
		task.Command.Cmd.Env = append(task.Command.Cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	task.Command.Cmd.Start()

	readPipe := func(resultChan chan string, pipe io.ReadCloser) {
		defer close(resultChan)

		scanner := bufio.NewScanner(pipe)
		scanner.Split(variableSplitFunc)
		for scanner.Scan() {
			message := scanner.Text()
			resultChan <- vtclean.Clean(message, false)
		}
	}

	stdoutChan := make(chan string, 1000)
	stderrChan := make(chan string, 1000)
	go readPipe(stdoutChan, stdoutPipe)
	go readPipe(stderrChan, stderrPipe)

	for {
		select {
		case stdoutMsg, ok := <-stdoutChan:
			if ok {
				// it seems that we are getting a bit behind... burn off elements without showing them on the screen
				if len(stdoutChan) > 100 {
					continue
				}

				if task.Config.EventDriven {
					// this is event driven... (signal this event)
					owningResultChan <- event{Task: task, Status: statusRunning, Stdout: blue(stdoutMsg), ReturnCode: -1}
				} else {
					// on a polling interval... (do not create an event)
					task.Display.Values.Msg = blue(stdoutMsg)
				}
				task.LogChan <- log.LogItem{Name: task.Config.Name, Message: stdoutMsg + "\n"}

			} else {
				stdoutChan = nil
			}
		case stderrMsg, ok := <-stderrChan:
			if ok {

				if task.Config.EventDriven {
					// either this is event driven... (signal this event)
					owningResultChan <- event{Task: task, Status: statusRunning, Stderr: red(stderrMsg), ReturnCode: -1}
				} else {
					// or on a polling interval... (do not create an event)
					task.Display.Values.Msg = red(stderrMsg)
				}
				task.LogChan <- log.LogItem{Name: task.Config.Name, Message: red(stderrMsg) + "\n"}
				task.ErrorBuffer.WriteString(stderrMsg + "\n")
			} else {
				stderrChan = nil
			}
		}
		if stdoutChan == nil && stderrChan == nil {
			break
		}
	}

	returnCode := 0
	returnCodeMsg := "unknown"
	if err := task.Command.Cmd.Wait(); err != nil {
		if exiterr, ok := err.(*exec.ExitError); ok {
			// The program has exited with an Exit code != 0
			if status, ok := exiterr.Sys().(syscall.WaitStatus); ok {
				returnCode = status.ExitStatus()
			}
		} else {
			returnCode = -1
			returnCodeMsg = "Failed to run: " + err.Error()
			owningResultChan <- event{Task: task, Status: StatusError, Stderr: returnCodeMsg, ReturnCode: returnCode}
			task.LogChan <- log.LogItem{Name: task.Config.Name, Message: red(returnCodeMsg) + "\n"}
			task.ErrorBuffer.WriteString(returnCodeMsg + "\n")
		}
	}
	task.Command.StopTime = time.Now()

	log.LogToMain("Completed Task: "+task.Config.Name+" (rc:"+strconv.Itoa(returnCode)+")", infoFormat)

	// close the write end of the pipe since the child shell is positively no longer writting to it
	task.Command.Cmd.ExtraFiles[0].Close()
	data, err := ioutil.ReadAll(task.Command.EnvReadFile)
	utils.CheckError(err, "Could not read env vars from child shell")

	if environment != nil {
		lines := strings.Split(string(data[:]), "\n")
		for _, line := range lines {
			fields := strings.SplitN(strings.TrimSpace(line), "=", 2)
			if len(fields) == 2 {
				environment[fields[0]] = fields[1]
			} else if len(fields) == 1 {
				environment[fields[0]] = ""
			}
		}
	}

	if returnCode == 0 || task.Config.IgnoreFailure {
		owningResultChan <- event{Task: task, Status: StatusSuccess, Complete: true, ReturnCode: returnCode}
	} else {
		owningResultChan <- event{Task: task, Status: StatusError, Complete: true, ReturnCode: returnCode}
		if task.Config.StopOnFailure {
			ExitSignaled = true
		}
	}
}

// Pave prints the initial task (and child task) formatted status to the screen using newline characters to advance rows (not ansi control codes)
func (task *Task) Pave() {
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
		task.display()
	}

	for line := 0; line < len(task.Children); line++ {
		task.Children[line].Display.Values = lineInfo{Status: statusPending.Color("i"), Title: task.Children[line].Config.Name}
		task.Children[line].display()
	}
}

// startAvailableTasks will kick start the maximum allowed number of commands (both primary and child task commands). Repeated invocation will iterate to new commands (and not repeat already completed commands)
func (task *Task) startAvailableTasks(environment map[string]string) {
	// Note that the parent task result channel and waiter are used for all Tasks and child Tasks
	if task.Config.CmdString != "" && !task.Command.Started && task.invoker.RunningTasks < config.Config.Options.MaxParallelCmds {
		go task.runSingleCmd(task.resultChan, &task.waiter, environment)
		task.Command.Started = true
		task.invoker.RunningTasks++
	}
	for ; task.invoker.RunningTasks < config.Config.Options.MaxParallelCmds && task.lastStartedTask < len(task.Children); task.lastStartedTask++ {
		go task.Children[task.lastStartedTask].runSingleCmd(task.resultChan, &task.waiter, nil)
		task.Children[task.lastStartedTask].Command.Started = true
		task.invoker.RunningTasks++
	}
}

// Completed marks a task command as being completed
func (task *Task) Completed(rc int) {
	task.Command.Complete = true
	task.Command.ReturnCode = rc
	close(task.LogChan)

	task.invoker.CompletedTasks = append(task.invoker.CompletedTasks, task)
	config.Config.CommandTimeCache[task.Config.CmdString] = task.Command.StopTime.Sub(task.Command.StartTime)
	task.invoker.RunningTasks--
}

// listenAndDisplay updates the screen frame with the latest task and child task updates as they occur (either in realtime or in a polling loop). Returns when all child processes have been completed.
func (task *Task) listenAndDisplay(environment map[string]string) {
	scr := NewScreen()
	// just wait for stuff to come back

	for task.invoker.RunningTasks > 0 {
		select {
		case <-Ticker.C:
			spinner.Next()

			if task.Config.CmdString != "" {
				if !task.Command.Complete && task.Command.Started {
					task.Display.Values.Prefix = spinner.Current()
					task.Display.Values.Eta = task.CurrentEta()
				}
				task.display()
			}

			for _, taskObj := range task.Children {
				if !taskObj.Command.Complete && taskObj.Command.Started {
					taskObj.Display.Values.Prefix = spinner.Current()
					taskObj.Display.Values.Eta = taskObj.CurrentEta()
				}
				taskObj.display()
			}

			// update the summary line
			if config.Config.Options.ShowSummaryFooter {
				scr.DisplayFooter(footer(statusPending, "", task.invoker))
			}

		case msgObj := <-task.resultChan:
			eventTask := msgObj.Task

			// update the state before displaying...
			if msgObj.Complete {
				eventTask.Completed(msgObj.ReturnCode)
				task.startAvailableTasks(environment)
				task.status = msgObj.Status
				if msgObj.Status == StatusError {
					// keep note of the failed task for an after task report
					task.FailedChildren++
					task.invoker.FailedTasks = append(task.invoker.FailedTasks, eventTask)
				}
			}

			if !eventTask.Config.ShowTaskOutput {
				msgObj.Stderr = ""
				msgObj.Stdout = ""
			}

			if msgObj.Stderr != "" {
				eventTask.Display.Values = lineInfo{Status: msgObj.Status.Color("i"), Title: eventTask.Config.Name, Msg: msgObj.Stderr, Prefix: spinner.Current(), Eta: eventTask.CurrentEta()}
			} else {
				eventTask.Display.Values = lineInfo{Status: msgObj.Status.Color("i"), Title: eventTask.Config.Name, Msg: msgObj.Stdout, Prefix: spinner.Current(), Eta: eventTask.CurrentEta()}
			}

			eventTask.display()

			// update the summary line
			if config.Config.Options.ShowSummaryFooter {
				scr.DisplayFooter(footer(statusPending, "", task.invoker))
			} else {
				scr.MovePastFrame(false)
			}

		}

	}

	if !ExitSignaled {
		task.waiter.Wait()
	}

}

// Run will run the current Tasks primary command and/or all child commands. When execution has completed, the screen frame will advance.
func (task *Task) Run(environment map[string]string) {

	var message bytes.Buffer

	if !config.Config.Options.SingleLineDisplay {
		task.Pave()
	}
	task.startAvailableTasks(environment)
	task.listenAndDisplay(environment)

	scr := NewScreen()
	hasHeader := len(task.Children) > 0 && !config.Config.Options.SingleLineDisplay
	collapseSection := task.Config.CollapseOnCompletion && hasHeader && task.FailedChildren == 0

	// complete the proc group status
	if hasHeader {
		message.Reset()
		collapseSummary := ""
		if collapseSection {
			collapseSummary = purple(" (" + strconv.Itoa(len(task.Children)) + " Tasks hidden)")
		}
		task.Display.Template.Execute(&message, lineInfo{Status: task.status.Color("i"), Title: task.Config.Name + collapseSummary, Prefix: config.Config.Options.BulletChar})
		scr.DisplayHeader(message.String())
	}

	// collapse sections or parallel Tasks...
	if collapseSection {

		// head to the top of the section (below the header) and erase all lines
		scr.EraseBelowHeader()

		// head back to the top of the section
		scr.MoveCursorToFirstLine()
	} else {
		// ... or this is a single task or configured not to collapse

		// instead, leave all of the text on the screen...
		// ...reset the cursor to the bottom of the section
		scr.MovePastFrame(false)
	}
}


