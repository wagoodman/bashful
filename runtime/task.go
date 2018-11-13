// Copyright © 2018 Alex Goodman
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package runtime

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"text/template"
	"time"

	"github.com/lunixbochs/vtclean"
	color "github.com/mgutz/ansi"
	terminal "github.com/wayneashleyberry/terminal-dimensions"
	"github.com/wagoodman/bashful/config"
	"github.com/wagoodman/bashful/utils"
	"github.com/wagoodman/bashful/log"
	"github.com/google/uuid"
)

var (
	sudoPassword       string
	exitSignaled       bool
	startTime          time.Time
	summaryTemplate, _     = template.New("summary line").Parse(` {{.Status}}    ` + color.Reset + ` {{printf "%-16s" .Percent}}` + color.Reset + ` {{.Steps}}{{.Errors}}{{.Msg}}{{.Split}}{{.Runtime}}{{.Eta}}`)
)


var (
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
	task := Task{
		Id: uuid.New(),
		Config: taskConfig,
	}
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
	task.Command = newCommand(task.Config)
	if eta, ok := config.Config.CommandTimeCache[task.Config.CmdString]; ok {
		task.Command.addEstimatedRuntime(eta)
	}

	task.Display.Template = lineDefaultTemplate
	task.Display.Index = displayIdx
	task.ErrorBuffer = bytes.NewBufferString("")

	task.events = make(chan event)
	task.status = statusPending
}

func (task *Task) UpdateExec(execpath string) {
	if task.Config.CmdString == "" {
		task.Config.CmdString = config.Config.Options.ExecReplaceString
	}
	task.Config.CmdString = strings.Replace(task.Config.CmdString, config.Config.Options.ExecReplaceString, execpath, -1)
	task.Config.URL = strings.Replace(task.Config.URL, config.Config.Options.ExecReplaceString, execpath, -1)

	task.Command = newCommand(task.Config)
	if eta, ok := config.Config.CommandTimeCache[task.Config.CmdString]; ok {
		task.Command.addEstimatedRuntime(eta)
	}
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
			task.Display.Values.Msg = utils.Red("Exited with error (" + strconv.Itoa(task.Command.ReturnCode) + ")")
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
	log.LogToMain("Started Task: "+task.Config.Name, log.StyleInfo)

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
					owningResultChan <- event{Task: task, Status: statusRunning, Stdout: utils.Blue(stdoutMsg), ReturnCode: -1}
				} else {
					// on a polling interval... (do not create an event)
					task.Display.Values.Msg = utils.Blue(stdoutMsg)
				}
				task.LogChan <- log.LogItem{Name: task.Config.Name, Message: stdoutMsg + "\n"}

			} else {
				stdoutChan = nil
			}
		case stderrMsg, ok := <-stderrChan:
			if ok {

				if task.Config.EventDriven {
					// either this is event driven... (signal this event)
					owningResultChan <- event{Task: task, Status: statusRunning, Stderr: utils.Red(stderrMsg), ReturnCode: -1}
				} else {
					// or on a polling interval... (do not create an event)
					task.Display.Values.Msg = utils.Red(stderrMsg)
				}
				task.LogChan <- log.LogItem{Name: task.Config.Name, Message: utils.Red(stderrMsg) + "\n"}
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
			task.LogChan <- log.LogItem{Name: task.Config.Name, Message: utils.Red(returnCodeMsg) + "\n"}
			task.ErrorBuffer.WriteString(returnCodeMsg + "\n")
		}
	}
	task.Command.StopTime = time.Now()

	log.LogToMain("Completed Task: "+task.Config.Name+" (rc:"+strconv.Itoa(returnCode)+")", log.StyleInfo)

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
			exitSignaled = true
		}
	}
}

// startAvailableTasks will kick start the maximum allowed number of commands (both primary and child task commands). Repeated invocation will iterate to new commands (and not repeat already completed commands)
func (task *Task) startAvailableTasks(environment map[string]string) {
	// Note that the parent task result channel and waiter are used for all Tasks and child Tasks
	if task.Config.CmdString != "" && !task.Command.Started && task.Executor.RunningTasks < config.Config.Options.MaxParallelCmds {
		go task.runSingleCmd(task.events, &task.waiter, environment)
		task.Command.Started = true
		task.Executor.RunningTasks++
	}
	for ; task.Executor.RunningTasks < config.Config.Options.MaxParallelCmds && task.lastStartedTask < len(task.Children); task.lastStartedTask++ {
		go task.Children[task.lastStartedTask].runSingleCmd(task.events, &task.waiter, nil)
		task.Children[task.lastStartedTask].Command.Started = true
		task.Executor.RunningTasks++
	}
}

// Completed marks a task command as being completed
func (task *Task) Completed(rc int) {
	task.Command.Complete = true
	task.Command.ReturnCode = rc
	close(task.LogChan)

	task.Executor.CompletedTasks = append(task.Executor.CompletedTasks, task)
	config.Config.CommandTimeCache[task.Config.CmdString] = task.Command.StopTime.Sub(task.Command.StartTime)
	task.Executor.RunningTasks--
}

// listen updates the screen frame with the latest task and child task updates as they occur (either in realtime or in a polling loop). Returns when all child processes have been completed.
func (task *Task) listen(environment map[string]string) {
	// scr := GetScreen()
	// just wait for stuff to come back

	for task.Executor.RunningTasks > 0 {
	    msgObj := <-task.events
	 	for _, handler := range task.Executor.eventHandlers {
			handler.onEvent(task, msgObj)
		}
	}
	for _, handler := range task.Executor.eventHandlers {
		handler.unregister(task)
	}

	if !exitSignaled {
		task.waiter.Wait()
	}

}

// Run will run the current Tasks primary command and/or all child commands. When execution has completed, the screen frame will advance.
func (task *Task) Run(environment map[string]string) {

	var message bytes.Buffer

	// todo: replace this!!!!!!
	// if !config.Config.Options.SingleLineDisplay {
	// 	task.Pave()
	// }
	for _, handler := range task.Executor.eventHandlers {
		handler.register(task)
	}

	task.startAvailableTasks(environment)
	task.listen(environment)

	scr := GetScreen()
	hasHeader := len(task.Children) > 0 && !config.Config.Options.SingleLineDisplay
	collapseSection := task.Config.CollapseOnCompletion && hasHeader && task.FailedChildren == 0

	// complete the proc group status
	if hasHeader {
		message.Reset()
		collapseSummary := ""
		if collapseSection {
			collapseSummary = utils.Purple(" (" + strconv.Itoa(len(task.Children)) + " Tasks hidden)")
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


