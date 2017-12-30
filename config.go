package main

import (
	"encoding/gob"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"time"

	"gopkg.in/yaml.v2"
)

// config represents a superset of options parsed from the user yaml file (or derived from user values)
var config struct {
	// Options is a global set of values to be applied to all tasks
	Options          OptionsConfig `yaml:"config"`

	// TaskConfigs is a list of task definitions and their metadata
	TaskConfigs      []TaskConfig  `yaml:"tasks"`

	// cachePath is the dir path to place any temporary files
	cachePath        string

	// logCachePath is the dir path to place temporary logs
	logCachePath     string

	// etaCachePath is the file path for per-task ETA values (derived from a tasks CmdString)
	etaCachePath     string

	// totalEtaSeconds is the calculated ETA given the tree of tasks to execute
	totalEtaSeconds  float64

	// commandTimeCache is the task CmdString-to-ETASeconds for any previously run command (read from etaCachePath)
	commandTimeCache map[string]time.Duration
}

// OptionsConfig is the set of values to be applied to all tasks or affect general behavior
type OptionsConfig struct {
	// BulletChar is a character (or short string) that should prefix any displayed task name
	BulletChar           string  `yaml:"bullet-char"`

	// CollapseOnCompletion indicates when a task with child tasks should be "rolled up" into a single line after all tasks have been executed
	CollapseOnCompletion bool    `yaml:"collapse-on-completion"`

	// EventDriven indicates if the screen should be updated on any/all task stdout/stderr events or on a polling schedule
	EventDriven          bool    `yaml:"event-driven"`

	// IgnoreFailure indicates when no errors should be registered (all task command non-zero return codes will be treated as a zero return code)
	IgnoreFailure        bool    `yaml:"ignore-failure"`

	// LogPath is simply the filepath to write all main log entries
	LogPath              string  `yaml:"log-path"`

	// MaxParallelCmds indicates the most number of parallel commands that should be run at any one time
	MaxParallelCmds      int     `yaml:"max-parallel-commands"`

	// ReplicaReplaceString is a char or short string that is replaced with values given by a tasks "for-each" configuration
	ReplicaReplaceString string  `yaml:"replica-replace-pattern"`

	// ShowSummaryErrors places the total number of errors in the summary footer
	ShowSummaryErrors bool       `yaml:"show-summary-errors"`

	// ShowSummaryFooter shows or hides the summary footer
	ShowSummaryFooter    bool    `yaml:"show-summary-footer"`

	// ShowFailureReport shows or hides the detailed report of all failed tasks after program execution
	ShowFailureReport    bool    `yaml:"show-failure-report"`

	// ShowSummarySteps places the "[ number of steps completed / total steps]" in the summary footer
	ShowSummarySteps bool        `yaml:"show-summary-steps"`

	// ShowSummaryTimes places the Runtime and ETA for the entire program execution in the summary footer
	ShowSummaryTimes     bool    `yaml:"show-summary-times"`

	// ShowTaskEta places the ETA for individual tasks on each task line (only while running)
	ShowTaskEta          bool    `yaml:"show-task-times"`

	// ShowTaskOutput shows or hides a tasks command stdout/stderr while running
	ShowTaskOutput       bool    `yaml:"show-task-output"`

	// StopOnFailure indicates to halt further program execution if a task command has a non-zero return code
	StopOnFailure        bool    `yaml:"stop-on-failure"`

	// UpdateInterval is the time in seconds that the screen should be refreshed (only if EventDriven=false)
	UpdateInterval       float64 `yaml:"update-interval"`
}

// NewOptionsConfig creates a new OptionsConfig populated with sane default values
func NewOptionsConfig() (obj OptionsConfig) {
	obj.BulletChar = "•"
	obj.EventDriven = true
	obj.IgnoreFailure = false
	obj.MaxParallelCmds = 4
	obj.ReplicaReplaceString = "?"
	obj.ShowFailureReport = true
	obj.ShowSummaryErrors = false
	obj.ShowSummaryFooter = true
	obj.ShowSummarySteps = true
	obj.ShowSummaryTimes = true
	obj.ShowTaskEta = false
	obj.ShowTaskOutput = true
	obj.StopOnFailure = true
	obj.UpdateInterval = -1
	return obj
}

// UnmarshalYAML parses and creates a OptionsConfig from a given user yaml string
func (options *OptionsConfig) UnmarshalYAML(unmarshal func(interface{}) error) error {
	type defaults OptionsConfig
	defaultValues := defaults(NewOptionsConfig())

	if err := unmarshal(&defaultValues); err != nil {
		return err
	}

	*options = OptionsConfig(defaultValues)

	// the global options must be available when parsing the task yaml (does order matter?)
	config.Options = *options
	return nil
}

// TaskConfig represents a task definition and all metadata (Note: this is not the task runtime object)
type TaskConfig struct {
	// Name is the display name of the task (if not provided, then CmdString is used)
	Name                 string       `yaml:"name"`

	// CmdString is the bash command to invoke when "running" this task
	CmdString            string       `yaml:"cmd"`

	// CollapseOnCompletion indicates when a task with child tasks should be "rolled up" into a single line after all tasks have been executed
	CollapseOnCompletion bool         `yaml:"collapse-on-completion"`

	// EventDriven indicates if the screen should be updated on any/all task stdout/stderr events or on a polling schedule
	EventDriven          bool         `yaml:"event-driven"`

	// ForEach is a list of strings that will be used to make replicas if the current task (tailored Name/CmdString replacements are handled via the 'ReplicaReplaceString' option)
	ForEach              []string     `yaml:"for-each"`

	// IgnoreFailure indicates when no errors should be registered (all task command non-zero return codes will be treated as a zero return code)
	IgnoreFailure        bool         `yaml:"ignore-failure"`

	// ParallelTasks is a list of child tasks that should be run in concurrently with one another
	ParallelTasks        []TaskConfig `yaml:"parallel-tasks"`

	// ShowTaskOutput shows or hides a tasks command stdout/stderr while running
	ShowTaskOutput       bool         `yaml:"show-output"`

	// StopOnFailure indicates to halt further program execution if a task command has a non-zero return code
	StopOnFailure        bool         `yaml:"stop-on-failure"`
}

// NewTaskConfig creates a new TaskConfig populated with sane default values (derived from the global OptionsConfig)
func NewTaskConfig() (obj TaskConfig) {
	obj.IgnoreFailure = config.Options.IgnoreFailure
	obj.StopOnFailure = config.Options.StopOnFailure
	obj.ShowTaskOutput = config.Options.ShowTaskOutput
	obj.EventDriven = config.Options.EventDriven
	obj.CollapseOnCompletion = config.Options.CollapseOnCompletion
	return obj
}

// UnmarshalYAML parses and creates a TaskConfig from a given user yaml string
func (task *TaskConfig) UnmarshalYAML(unmarshal func(interface{}) error) error {
	type defaults TaskConfig
	defaultValues := defaults(NewTaskConfig())

	if err := unmarshal(&defaultValues); err != nil {
		return err
	}

	*task = TaskConfig(defaultValues)
	return nil
}

// MinMax returns the min and max values from an array of float64 values
func MinMax(array []float64) (float64, float64, error) {
	if len(array) == 0 {
		return 0, 0, errors.New("no min/max of empty array")
	}
	var max = array[0]
	var min = array[0]
	for _, value := range array {
		if max < value {
			max = value
		}
		if min > value {
			min = value
		}
	}
	return min, max, nil
}

// removeOneValue removes the first matching value from the given array of float64 values
func removeOneValue(slice []float64, value float64) []float64 {
	for index, arrValue := range slice {
		if arrValue == value {
			return append(slice[:index], slice[index+1:]...)
		}
	}
	return slice
}

// readTimeCache fetches and reads a cache file from disk containing CmdString-to-ETASeconds. Note: this this must be done before fetching/parsing the run.yaml
func readTimeCache() {
	cwd, err := os.Getwd()
	CheckError(err, "Unable to get CWD.")

	config.cachePath = path.Join(cwd, ".bashful")
	config.logCachePath = path.Join(config.cachePath, "logs")
	config.etaCachePath = path.Join(config.cachePath, "eta")

	// create the cache path and log dir if they do not already exist
	if _, err := os.Stat(config.cachePath); os.IsNotExist(err) {
		os.Mkdir(config.cachePath, 0755)
	}
	if _, err := os.Stat(config.logCachePath); os.IsNotExist(err) {
		os.Mkdir(config.logCachePath, 0755)
	}

	config.commandTimeCache = make(map[string]time.Duration)
	if doesFileExist(config.etaCachePath) {
		err := Load(config.etaCachePath, &config.commandTimeCache)
		CheckError(err, "Unable to load command eta cache.")
	}
}

// readRunYaml fetches and reads the user given yaml file from disk and populates the global config object
func readRunYaml(userYamlPath string) {
	// fetch and parse the run.yaml user file...
	config.Options = NewOptionsConfig()

	yamlString, err := ioutil.ReadFile(userYamlPath)

	CheckError(err, "Unable to read yaml config.")

	err = yaml.Unmarshal(yamlString, &config)
	if err != nil {
		fmt.Println(red("Error: Unable to parse '" + userYamlPath + "'"))
		fmt.Println(err)
		exit(1)
	}
}

// createTasks is responsible for reading all parsed TaskConfigs and generating a list of Task runtime objects to later execute
func createTasks() (finalTasks []*Task) {

	// initialize tasks with default values
	for index := range config.TaskConfigs {
		nextDisplayIdx = 0
		taskConfig := &config.TaskConfigs[index]

		// finalize task by appending to the set of final tasks
		if len(taskConfig.ForEach) > 0 {
			taskName, taskCmdString := taskConfig.Name, taskConfig.CmdString
			for _, replicaValue := range taskConfig.ForEach {
				taskConfig.Name = taskName
				taskConfig.CmdString = taskCmdString
				task := NewTask(*taskConfig, nextDisplayIdx, replicaValue)
				finalTasks = append(finalTasks, &task)
			}
		} else {
			task := NewTask(*taskConfig, nextDisplayIdx, "")
			finalTasks = append(finalTasks, &task)
		}
	}

	// now that all tasks have been inflated, set the total eta
	for _, task := range finalTasks {
		config.totalEtaSeconds += task.EstimateRuntime()
	}

	// replace the current config with the inflated list of final tasks
	return finalTasks
}

// ReadConfig is the entrypoint for all config fetching and parsing. This returns a list of Task runtime objects to execute.
func ReadConfig(userYamlPath string) []*Task {
	readTimeCache()
	readRunYaml(userYamlPath)

	if config.Options.LogPath != "" {
		setupLogging()
	}

	return createTasks()
}

// Save encodes a generic object via Gob to the given file path
func Save(path string, object interface{}) error {
	file, err := os.Create(path)
	if err == nil {
		encoder := gob.NewEncoder(file)
		encoder.Encode(object)
	}
	file.Close()
	return err
}

// Load decodes via Gob the contents of the given file to an object
func Load(path string, object interface{}) error {
	file, err := os.Open(path)
	if err == nil {
		decoder := gob.NewDecoder(file)
		err = decoder.Decode(object)
	}
	file.Close()
	return err
}
