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
	"bytes"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/wagoodman/bashful/pkg/config"
)

// EventHandler represents a type that can listen to Task events managed by the Executor
type EventHandler interface {
	// Register a task with the handler
	Register(task *Task)
	// Unregister a task from the handler
	Unregister(task *Task)
	// OnEvent is invoked by a task when execution state changes
	OnEvent(task *Task, e TaskEvent)
	// Close the handler
	Close()
	// AddRuntimeData makes realtime statistics available to the handler
	AddRuntimeData(data *TaskStatistics)
}

type Client struct {
	// Config contains the runtime configuration for the application
	Config *config.Config

	// Executor is the task invoker
	Executor *Executor
}

type Executor struct {
	// Environment is a mapping of all environment variables passed to each task on execution (except parallel tasks)
	Environment map[string]string

	eventHandlers []EventHandler

	config *config.Config

	// cmdEtaCache is the task CmdString-to-ETASeconds for any previously run command (read from EtaCachePath)
	cmdEtaCache map[string]time.Duration

	// Tasks is a list of all Task objects that will be invoked
	Tasks []*Task

	// Statistics contains runtime statistics of all planned tasks
	Statistics *TaskStatistics
}

type TaskStatistics struct {
	// Failed is a list of Task objects with non-zero return codes upon invocation
	Failed []*Task

	// Running indicates the number of actively running Tasks
	Running int

	// Completed is a list of Task objects that have been invoked (regardless of the return code value)
	Completed []*Task

	// Total indicates the number of tasks that can be run (Note: this is not necessarily the same number of tasks planned to be run)
	Total int
}

// Task is a runtime object derived from the TaskConfig (parsed from the user yaml) and contains everything needed to Execute, track, and display the task.
type Task struct {
	Id uuid.UUID

	// Config is the user-defined values parsed from the run yaml
	Config config.TaskConfig

	Options *config.Options

	// Command represents all non-Config items used to Execute and track task progress
	Command command

	// Children is a list of all sub-Tasks that should be run concurrently
	Children []*Task

	// events is a channel where all raw command events are queued to
	events chan TaskEvent

	// waiter is a synchronization object which returns when all child task command executions have been markCompleted
	waiter sync.WaitGroup

	// TaskStatus is the last known TaskStatus value that represents the entire list of child commands
	Status TaskStatus

	// Started indicates whether the Task has been attempted to run
	Started bool

	// Completed indicates whether the Task has been finished execution
	Completed bool

	// FailedChildren is a list of Tasks with a non-zero return value
	FailedChildren int
}

// command represents all non-Config items used to Execute and track task progress
type command struct {
	// Cmd is the object used to Execute the given user CmdString to a sub-shell
	Cmd *exec.Cmd

	// TempExecFromURL is the path to a temporary file downloaded from a TaskConfig url reference
	TempExecFromURL string

	// startTime indicates when the Cmd was started
	StartTime time.Time

	// StopTime indicates when the Cmd markCompleted execution
	StopTime time.Time

	// EstimatedRuntime indicates the expected runtime for the given command (based off of cached values from previous runs)
	EstimatedRuntime time.Duration

	// ReturnCode is simply the value returned from the child process after Cmd execution
	ReturnCode int

	// EnvReadFile is an extra pipe given to the child shell process for exfiltrating env vars back up to bashful (to provide as input for future Tasks)
	EnvReadFile *os.File

	// Environment is a list of env vars from the exited child process
	Environment map[string]string

	// errorBuffer contains all stderr lines generated from the executed command (used to generate the task report)
	errorBuffer *bytes.Buffer
}

// TaskStatus represents whether a task command is about to run, already running, or has completed (in which case, was it successful or not)
type TaskStatus int32

// TaskEvent represents an output from stdout/stderr during command execution or when a command has markCompleted
type TaskEvent struct {
	// Task is the task which the command was run from
	Task *Task

	// Status is the current pending/running/error/success TaskStatus of the command
	Status TaskStatus

	// Stdout is a single line from standard out (optional)
	Stdout string

	// Stderr is a single line from standard error (optional)
	Stderr string

	// Completed indicates if the command has exited
	Complete bool

	// todo: remove return code from an event
	// ReturnCode is the sub-process return code value upon completion
	ReturnCode int
}
