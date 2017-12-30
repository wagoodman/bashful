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

var config struct {
	Options          OptionsConfig `yaml:"config"`
	TaskConfigs      []TaskConfig  `yaml:"tasks"`
	logCachePath     string
	cachePath        string
	etaCachePath     string
	totalEtaSeconds  float64
	commandTimeCache map[string]time.Duration
}

type OptionsConfig struct {
	BulletChar           string  `yaml:"bullet-char"`
	StopOnFailure        bool    `yaml:"stop-on-failure"`
	ShowStepSummary      bool    `yaml:"show-summary-steps"`
	ShowErrorSummary     bool    `yaml:"show-summary-errors"`
	ShowSummaryFooter    bool    `yaml:"show-summary-footer"`
	ShowFailureReport    bool    `yaml:"show-failure-report"`
	LogPath              string  `yaml:"log-path"`
	MaxParallelCmds      int     `yaml:"max-parallel-commands"`
	ReplicaReplaceString string  `yaml:"replica-replace-pattern"`
	ShowTaskEta          bool    `yaml:"show-task-times"`
	ShowTaskOutput       bool    `yaml:"show-task-output"`
	ShowSummaryTimes     bool    `yaml:"show-summary-times"`
	CollapseOnCompletion bool    `yaml:"collapse-on-completion"`
	UpdateInterval       float64 `yaml:"update-interval"`
	EventDriven          bool    `yaml:"event-driven"`
}

func defaultOptions() OptionsConfig {
	var defaultValues OptionsConfig
	defaultValues.StopOnFailure = true
	defaultValues.ShowSummaryFooter = true
	defaultValues.ShowErrorSummary = false
	defaultValues.ShowStepSummary = true
	defaultValues.ShowTaskOutput = true
	defaultValues.ShowFailureReport = true
	defaultValues.ReplicaReplaceString = "?"
	defaultValues.MaxParallelCmds = 4
	defaultValues.ShowSummaryTimes = true
	defaultValues.ShowTaskEta = false
	defaultValues.UpdateInterval = -1
	defaultValues.EventDriven = true
	defaultValues.BulletChar = "•"
	return defaultValues
}

func (options *OptionsConfig) UnmarshalYAML(unmarshal func(interface{}) error) error {
	type defaults OptionsConfig
	var defaultValues defaults
	defaultValues = defaults(defaultOptions())

	if err := unmarshal(&defaultValues); err != nil {
		return err
	}

	*options = OptionsConfig(defaultValues)
	// the global options must be available when parsing the task yaml (does order matter?)
	config.Options = *options
	return nil
}

type TaskConfig struct {
	Name                 string       `yaml:"name"`
	CmdString            string       `yaml:"cmd"`
	StopOnFailure        bool         `yaml:"stop-on-failure"`
	CollapseOnCompletion bool         `yaml:"collapse-on-completion"`
	EventDriven          bool         `yaml:"event-driven"`
	ShowTaskOutput       bool         `yaml:"show-output"`
	IgnoreFailure        bool         `yaml:"ignore-failure"`
	ParallelTasks        []TaskConfig `yaml:"parallel-tasks"`
	ForEach              []string     `yaml:"for-each"`
}

func (task *TaskConfig) UnmarshalYAML(unmarshal func(interface{}) error) error {
	type defaults TaskConfig
	var defaultValues defaults
	defaultValues.StopOnFailure = config.Options.StopOnFailure
	defaultValues.ShowTaskOutput = config.Options.ShowTaskOutput
	defaultValues.EventDriven = config.Options.EventDriven
	defaultValues.CollapseOnCompletion = config.Options.CollapseOnCompletion

	if err := unmarshal(&defaultValues); err != nil {
		return err
	}

	*task = TaskConfig(defaultValues)
	return nil
}

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

func removeOneValue(slice []float64, value float64) []float64 {
	for index, arrValue := range slice {
		if arrValue == value {
			return append(slice[:index], slice[index+1:]...)
		}
	}
	return slice
}

func readTimeCache() {
	// fetch the ETA cache from disk (this must be done before fetching/parsing the run.yaml)...
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

func readRunYaml(userYamlPath string) {
	// fetch and parse the run.yaml user file...
	config.Options = defaultOptions()

	yamlString, err := ioutil.ReadFile(userYamlPath)

	CheckError(err, "Unable to read yaml config.")

	err = yaml.Unmarshal(yamlString, &config)
	if err != nil {
		fmt.Println(red("Error: Unable to parse '" + userYamlPath + "'"))
		fmt.Println(err)
		exit(1)
	}
}

func createTasks() (finalTasks []*Task) {

	// initialize tasks with default values...
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
		config.totalEtaSeconds += task.EstimatedRuntime()
	}

	// replace the current config with the inflated list of final tasks
	return finalTasks
}

func ReadConfig(userYamlPath string) []*Task {
	readTimeCache()
	readRunYaml(userYamlPath)

	if config.Options.LogPath != "" {
		setupLogging()
	}

	return createTasks()
}

// Encode via Gob to file
func Save(path string, object interface{}) error {
	file, err := os.Create(path)
	if err == nil {
		encoder := gob.NewEncoder(file)
		encoder.Encode(object)
	}
	file.Close()
	return err
}

// Decode Gob file
func Load(path string, object interface{}) error {
	file, err := os.Open(path)
	if err == nil {
		decoder := gob.NewDecoder(file)
		err = decoder.Decode(object)
	}
	file.Close()
	return err
}
