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
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/lunixbochs/vtclean"
	"github.com/wagoodman/bashful/config"
	"github.com/wagoodman/bashful/utils"
	terminal "github.com/wayneashleyberry/terminal-dimensions"
)

var (
	sudoPassword string
	exitSignaled bool
)

const (
	StatusRunning TaskStatus = iota
	StatusPending
	StatusSuccess
	StatusError
)

// NewTask creates a new task in the context of the user configuration at a particular screen location (row)
func NewTask(taskConfig config.TaskConfig, executor *Executor, runtimeOptions *config.Options) *Task {
	task := Task{
		Id:       uuid.New(),
		Config:   taskConfig,
		Options:  runtimeOptions,
		Executor: executor,
	}

	task.Command = newCommand(task.Config)
	task.ErrorBuffer = bytes.NewBufferString("")
	task.events = make(chan TaskEvent)
	task.Status = StatusPending

	for subIndex := range taskConfig.ParallelTasks {
		subTaskConfig := &taskConfig.ParallelTasks[subIndex]

		subTask := NewTask(*subTaskConfig, executor, runtimeOptions)
		task.Children = append(task.Children, subTask)
	}

	return &task
}

func (task *Task) UpdateExec(execpath string) {
	// todo: this needs to be rethought
	if task.Config.CmdString == "" {
		task.Config.CmdString = task.Options.ExecReplaceString
	}
	task.Config.CmdString = strings.Replace(task.Config.CmdString, task.Options.ExecReplaceString, execpath, -1)
	task.Config.URL = strings.Replace(task.Config.URL, task.Options.ExecReplaceString, execpath, -1)

	task.Command = newCommand(task.Config)
	if eta, ok := task.Executor.CommandTimeCache[task.Config.CmdString]; ok {
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
	var remainingParallelTasks = task.Options.MaxParallelCmds

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
func (task *Task) runSingleCmd(owningResultChan chan TaskEvent, owningWaiter *sync.WaitGroup, environment map[string]string) {

	task.Command.StartTime = time.Now()

	owningResultChan <- TaskEvent{Task: task, Status: StatusRunning, ReturnCode: -1}
	owningWaiter.Add(1)
	defer owningWaiter.Done()

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

				// todo: we should always throw the TaskEvent? let the TaskEvent handler deal with TaskEvent/polling...
				if task.Config.EventDriven {
					// this is TaskEvent driven... (signal this TaskEvent)
					owningResultChan <- TaskEvent{Task: task, Status: StatusRunning, Stdout: utils.Blue(stdoutMsg), ReturnCode: -1}
				}
				// else {
				// 	// on a polling interval... (do not create an TaskEvent)
				// 	task.Display.Values.Msg = utils.Blue(stdoutMsg)
				// }

			} else {
				stdoutChan = nil
			}
		case stderrMsg, ok := <-stderrChan:
			if ok {

				// todo: we should always throw the TaskEvent? let the TaskEvent handler deal with TaskEvent/polling...
				if task.Config.EventDriven {
					// either this is TaskEvent driven... (signal this TaskEvent)
					owningResultChan <- TaskEvent{Task: task, Status: StatusRunning, Stderr: utils.Red(stderrMsg), ReturnCode: -1}
				}
				// else {
				// 	// or on a polling interval... (do not create an TaskEvent)
				// 	task.Display.Values.Msg = utils.Red(stderrMsg)
				// }
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
			owningResultChan <- TaskEvent{Task: task, Status: StatusError, Stderr: returnCodeMsg, ReturnCode: returnCode}
			task.ErrorBuffer.WriteString(returnCodeMsg + "\n")
		}
	}
	task.Command.StopTime = time.Now()

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
		owningResultChan <- TaskEvent{Task: task, Status: StatusSuccess, Complete: true, ReturnCode: returnCode}
	} else {
		owningResultChan <- TaskEvent{Task: task, Status: StatusError, Complete: true, ReturnCode: returnCode}
		if task.Config.StopOnFailure {
			exitSignaled = true
		}
	}
}

// startAvailableTasks will kick start the maximum allowed number of commands (both primary and child task commands). Repeated invocation will iterate to new commands (and not repeat already completed commands)
func (task *Task) startAvailableTasks(environment map[string]string) {
	// Note that the parent task result channel and waiter are used for all Tasks and child Tasks
	if task.Config.CmdString != "" && !task.Command.Started && task.Executor.RunningTasks < task.Options.MaxParallelCmds {
		go task.runSingleCmd(task.events, &task.waiter, environment)
		task.Command.Started = true
		task.Executor.RunningTasks++
	}
	for ; task.Executor.RunningTasks < task.Options.MaxParallelCmds && task.lastStartedTask < len(task.Children); task.lastStartedTask++ {
		go task.Children[task.lastStartedTask].runSingleCmd(task.events, &task.waiter, nil)
		task.Children[task.lastStartedTask].Command.Started = true
		task.Executor.RunningTasks++
	}
}

// completed marks a task command as being completed
func (task *Task) completed(rc int) {
	task.Command.Complete = true
	task.Command.ReturnCode = rc

	task.Executor.CompletedTasks = append(task.Executor.CompletedTasks, task)
	task.Executor.CommandTimeCache[task.Config.CmdString] = task.Command.StopTime.Sub(task.Command.StartTime)
	task.Executor.RunningTasks--
}

// Run will run the current Tasks primary command and/or all child commands. When execution has completed, the screen frame will advance.
func (task *Task) Run(environment map[string]string) {
	for _, handler := range task.Executor.eventHandlers {
		handler.Register(task)
	}

	task.startAvailableTasks(environment)

	for task.Executor.RunningTasks > 0 {
		msgObj := <-task.events

		// manage completed tasks...
		if msgObj.Complete {
			msgObj.Task.completed(msgObj.ReturnCode)
			task.startAvailableTasks(environment)

			task.Status = msgObj.Status

			if msgObj.Status == StatusError {
				// keep note of the failed task for an after task report
				task.FailedChildren++
				task.Executor.FailedTasks = append(task.Executor.FailedTasks, msgObj.Task)
			}
		}

		// notify all handlers...
		for _, handler := range task.Executor.eventHandlers {
			handler.OnEvent(task, msgObj)
		}
	}

	if !exitSignaled {
		task.waiter.Wait()
	}

	// we should be done with all tasks/subtasks at this point, unregister everything
	for _, subTask := range task.Children {
		for _, handler := range subTask.Executor.eventHandlers {
			handler.Unregister(subTask)
		}
	}
	for _, handler := range task.Executor.eventHandlers {
		handler.Unregister(task)
	}

}
