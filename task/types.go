package task

import (
	"github.com/wagoodman/bashful/config"
	"github.com/wagoodman/bashful/log"
	"os"
	"bytes"
	"sync"
	"os/exec"
	"time"
	"text/template"
)

// Task is a runtime object derived from the TaskConfig (parsed from the user yaml) and contains everything needed to execute, track, and display the task.
type Task struct {
	// Config is the user-defined values parsed from the run yaml
	Config config.TaskConfig

	// Display represents all non-Config items that control how the task line should be printed to the screen
	Display TaskDisplay

	// Command represents all non-Config items used to execute and track task progress
	Command TaskCommand

	// LogChan is a channel with event log items written to the temporary logfile
	LogChan chan log.LogItem

	// LogFile is the temporary log file where all formatted stdout/stderr events are recorded
	LogFile *os.File

	// ErrorBuffer contains all stderr lines generated from the executed command (used to generate the task report)
	ErrorBuffer *bytes.Buffer

	// Children is a list of all sub-tasks that should be run concurrently
	Children []*Task

	// lastStartedTask is the index of the last child task that was started
	lastStartedTask int

	// resultChan is a channel where all raw command events are queued to
	resultChan chan CmdEvent

	// waiter is a synchronization object which returns when all child task command executions have been completed
	waiter sync.WaitGroup

	// status is the last known status value that represents the entire list of child commands
	status CommandStatus

	// FailedTasks is a list of tasks with a non-zero return value
	FailedTasks []*Task
}

// TaskDisplay represents all non-Config items that control how the task line should be printed to the screen
type TaskDisplay struct {
	// Template is the single-line string template that should be used to display the status of a single task
	Template *template.Template

	// Index is the row within a screen frame to print the task template
	Index int

	// Values holds all template values that represent the task status
	Values LineInfo
}

// TaskCommand represents all non-Config items used to execute and track task progress
type TaskCommand struct {
	// Cmd is the object used to execute the given user CmdString to a sub-shell
	Cmd *exec.Cmd

	// TempExecFromURL is the path to a temporary file downloaded from a TaskConfig url reference
	TempExecFromURL string

	// StartTime indicates when the Cmd was started
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

	// EnvReadFile is an extra pipe given to the child shell process for exfiltrating env vars back up to bashful (to provide as input for future tasks)
	EnvReadFile *os.File

	// Environment is a list of env vars from the exited child process
	Environment map[string]string
}

// CommandStatus represents whether a task command is about to run, already running, or has completed (in which case, was it successful or not)
type CommandStatus int32

// CmdEvent represents an output from stdout/stderr during command execution or when a command has completed
type CmdEvent struct {
	// Task is the task which the command was run from
	Task *Task

	// Status is the current pending/running/error/success status of the command
	Status CommandStatus

	// Stdout is a single line from standard out (optional)
	Stdout string

	// Stderr is a single line from standard error (optional)
	Stderr string

	// Complete indicates if the command has exited
	Complete bool

	// ReturnCode is the sub-process return code value upon completion
	ReturnCode int
}

// LineInfo represents all template values that represent the task status
type LineInfo struct {
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
