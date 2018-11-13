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
	"github.com/wagoodman/bashful/config"
	"github.com/wagoodman/bashful/log"
	"os"
	"bytes"
	"sync"
	"os/exec"
	"time"
	"text/template"
	"github.com/google/uuid"
)

type EventHandler interface {
	register(task *Task)
	onEvent(task *Task, e event)
}

type Client struct {
	Options       config.OptionsConfig
	TaskConfigs   []config.TaskConfig
	Executor      *Executor
}

type Executor struct {
	environment map[string]string
	eventHandlers []EventHandler

	// Tasks is a list of all Task objects that will be invoked
	Tasks       []*Task

	// FailedTasks is a list of Task objects with non-zero return codes upon invocation
	FailedTasks []*Task

	// RunningTasks indicates the number of actively running Tasks
	RunningTasks int

	// CompletedTasks is a list of Task objects that have been invoked (regardless of the return code value)
	CompletedTasks []*Task

	// TotalTasks indicates the number of tasks that can be run (Note: this is not necessarily the same number of tasks planned to be run)
	TotalTasks int
}

// Task is a runtime object derived from the TaskConfig (parsed from the user yaml) and contains everything needed to execute, track, and display the task.
type Task struct {
	Id uuid.UUID

	// Config is the user-defined values parsed from the run yaml
	Config config.TaskConfig

	// Display represents all non-Config items that control how the task line should be printed to the screen
	Display display

	// Command represents all non-Config items used to execute and track task progress
	Command command

	// todo: do we need both logfile and logchan in a task?
	// LogChan is a channel with event log items written to the temporary logfile
	LogChan chan log.LogItem

	// LogFile is the temporary log file where all formatted stdout/stderr events are recorded
	LogFile *os.File

	// ErrorBuffer contains all stderr lines generated from the executed command (used to generate the task report)
	ErrorBuffer *bytes.Buffer

	// Children is a list of all sub-Tasks that should be run concurrently
	Children []*Task

	// lastStartedTask is the index of the last child task that was started
	lastStartedTask int

	// TODO: this should be removed from a task?
	// events is a channel where all raw command events are queued to
	events chan event

	// waiter is a synchronization object which returns when all child task command executions have been completed
	waiter sync.WaitGroup

	// status is the last known status value that represents the entire list of child commands
	status status

	// FailedTasks is a list of Tasks with a non-zero return value
	FailedChildren int

	Executor *Executor
}

// display represents all non-Config items that control how the task line should be printed to the screen
type display struct {
	// Template is the single-line string template that should be used to display the status of a single task
	Template *template.Template

	// Index is the row within a screen frame to print the task template
	Index int

	// Values holds all template values that represent the task status
	Values lineInfo
}

// command represents all non-Config items used to execute and track task progress
type command struct {
	// Cmd is the object used to execute the given user CmdString to a sub-shell
	Cmd *exec.Cmd

	// TempExecFromURL is the path to a temporary file downloaded from a TaskConfig url reference
	TempExecFromURL string

	// startTime indicates when the Cmd was started
	StartTime time.Time

	// StopTime indicates when the Cmd completed execution
	StopTime time.Time

	// EstimatedRuntime indicates the expected runtime for the given command (based off of cached values from previous runs)
	EstimatedRuntime time.Duration

	// Started indicates whether the Cmd has been attempted to run
	Started bool

	// Complete indicates whether the Cmd has been finished execution
	Complete bool

	// ReturnCode is simply the value returned from the child process after Cmd execution
	ReturnCode int

	// EnvReadFile is an extra pipe given to the child shell process for exfiltrating env vars back up to bashful (to provide as input for future Tasks)
	EnvReadFile *os.File

	// Environment is a list of env vars from the exited child process
	Environment map[string]string
}

// status represents whether a task command is about to run, already running, or has completed (in which case, was it successful or not)
type status int32

// event represents an output from stdout/stderr during command execution or when a command has completed
type event struct {
	// Task is the task which the command was run from
	Task *Task

	// Status is the current pending/running/error/success status of the command
	Status status

	// Stdout is a single line from standard out (optional)
	Stdout string

	// Stderr is a single line from standard error (optional)
	Stderr string

	// Complete indicates if the command has exited
	Complete bool

	// ReturnCode is the sub-process return code value upon completion
	ReturnCode int
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
