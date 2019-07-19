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

package config

import (
	mapset "github.com/deckarep/golang-set"
)

type stringArray []string

// Config represents a superset of options parsed from the user yaml file (or derived from user values)
type Config struct {
	Cli Cli

	// Options is a global set of values to be applied to all tasks
	Options Options `yaml:"config"`

	// TaskConfigs is a list of task definitions and their metadata
	TaskConfigs []TaskConfig `yaml:"tasks"`

	// CachePath is the dir path to place any temporary files
	CachePath string

	// LogCachePath is the dir path to place temporary logs
	LogCachePath string

	// EtaCachePath is the file path for per-task ETA values (derived from a tasks CmdString)
	EtaCachePath string

	// DownloadCachePath is the dir path to place downloaded resources (from url references)
	DownloadCachePath string

	// TotalEtaSeconds is the calculated ETA given the tree of tasks to execute
	TotalEtaSeconds float64
}

// Cli is the exhaustive set of all command line options available on bashful
type Cli struct {
	YamlPath               string
	RunTags                []string
	RunTagSet              mapset.Set
	ExecuteOnlyMatchedTags bool
	Args                   []string
}

// Options is the set of values to be applied to all tasks or affect general behavior
type Options struct {
	// BulletChar is a character (or short string) that should prefix any displayed task name
	BulletChar string `yaml:"bullet-char"`

	// Bundle is a list of relative file paths that should be included in a bashful bundle
	Bundle []string `yaml:"bundle"`

	// CollapseOnCompletion indicates when a task with child tasks should be "rolled up" into a single line after all tasks have been executed
	CollapseOnCompletion bool `yaml:"collapse-on-completion"`

	// ColorRunning is the color of the vertical progress bar when the task is running (# in the 256 palett)
	ColorRunning int `yaml:"running-status-color"`

	// ColorPending is the color of the vertical progress bar when the task is waiting to be ran (# in the 256 palett)
	ColorPending int `yaml:"pending-status-color"`

	// ColorSuccessg is the color of the vertical progress bar when the task has finished successfully (# in the 256 palett)
	ColorSuccess int `yaml:"success-status-color"`

	// ColorError is the color of the vertical progress bar when the task has failed (# in the 256 palett)
	ColorError int `yaml:"error-status-color"`

	// EventDriven indicates if the screen should be updated on any/all task stdout/stderr events or on a polling schedule
	EventDriven bool `yaml:"event-driven"`

	// ExecReplaceString is a char or short string that is replaced with the temporary executable path when using the 'url' task Config option
	ExecReplaceString string `yaml:"exec-replace-pattern"`

	// IgnoreFailure indicates when no errors should be registered (all task command non-zero return codes will be treated as a zero return code)
	IgnoreFailure bool `yaml:"ignore-failure"`

	// LogPath is simply the filepath to write all main log entries
	LogPath string `yaml:"log-path"`

	// MaxParallelCmds indicates the most number of parallel commands that should be run at any one time
	MaxParallelCmds int `yaml:"max-parallel-commands"`

	// ReplicaReplaceString is a char or short string that is replaced with values given by a tasks "for-each" configuration
	ReplicaReplaceString string `yaml:"replica-replace-pattern"`

	// ShowSummaryErrors places the total number of errors in the summary footer
	ShowSummaryErrors bool `yaml:"show-summary-errors"`

	// ShowSummaryFooter shows or hides the summary footer
	ShowSummaryFooter bool `yaml:"show-summary-footer"`

	// ShowFailureReport shows or hides the detailed report of all failed tasks after program execution
	ShowFailureReport bool `yaml:"show-failure-report"`

	// ShowSummarySteps places the "[ number of steps completed / total steps]" in the summary footer
	ShowSummarySteps bool `yaml:"show-summary-steps"`

	// ShowSummaryTimes places the Runtime and ETA for the entire program execution in the summary footer
	ShowSummaryTimes bool `yaml:"show-summary-times"`

	// ShowTaskEta places the ETA for individual tasks on each task line (only while running)
	ShowTaskEta bool `yaml:"show-task-times"`

	// ShowTaskOutput shows or hides a tasks command stdout/stderr while running
	ShowTaskOutput bool `yaml:"show-task-output"`

	// StopOnFailure indicates to halt further program execution if a task command has a non-zero return code
	StopOnFailure bool `yaml:"stop-on-failure"`

	// SingleLineDisplay indicates to show all bashful output in a single line (instead of a line per task + a summary line)
	SingleLineDisplay bool `yaml:"single-line"`

	// UpdateInterval is the time in seconds that the screen should be refreshed (only if EventDriven=false)
	UpdateInterval float64 `yaml:"update-interval"`
}

// TaskConfig represents a task definition and all metadata (Note: this is not the task runtime object)
type TaskConfig struct {
	// Name is the display name of the task (if not provided, then CmdString is used)
	Name string `yaml:"name"`

	// CmdString is the bash command to invoke when "running" this task
	CmdString string `yaml:"cmd"`

	// CwdString is current working directory
	CwdString string `yaml:"cwd"`

	// CollapseOnCompletion indicates when a task with child tasks should be "rolled up" into a single line after all tasks have been executed
	CollapseOnCompletion bool `yaml:"collapse-on-completion"`

	// EventDriven indicates if the screen should be updated on any/all task stdout/stderr events or on a polling schedule
	EventDriven bool `yaml:"event-driven"`

	// ForEach is a list of strings that will be used to make replicas if the current task (tailored Name/CmdString replacements are handled via the 'ReplicaReplaceString' option)
	ForEach []string `yaml:"for-each"`

	// IgnoreFailure indicates when no errors should be registered (all task command non-zero return codes will be treated as a zero return code)
	IgnoreFailure bool `yaml:"ignore-failure"`

	// Md5 is the expected hash value after digesting a downloaded file from a Url (only used with TaskConfig.Url)
	Md5 string `yaml:"md5"`

	// ParallelTasks is a list of child tasks that should be run in concurrently with one another
	ParallelTasks []TaskConfig `yaml:"parallel-tasks"`

	// ShowTaskOutput shows or hides a tasks command stdout/stderr while running
	ShowTaskOutput bool `yaml:"show-output"`

	// StopOnFailure indicates to halt further program execution if a task command has a non-zero return code
	StopOnFailure bool `yaml:"stop-on-failure"`

	// Sudo indicates that the given command should be run with the given sudo credentials
	Sudo bool `yaml:"sudo"`

	// Tags is a list of strings that is used to filter down which task are run at runtime
	Tags   stringArray `yaml:"tags"`
	TagSet mapset.Set

	// URL is the http/https link to a bash/executable resource
	URL string `yaml:"url"`
}
