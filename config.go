package main

import (
	"encoding/gob"
	"io/ioutil"
	"os"
	"path"
	"time"

	"gopkg.in/yaml.v2"
)

var config struct {
	Options          OptionsConfig `yaml:"config"`
	Tasks            []Task        `yaml:"tasks"`
	logCachePath     string
	cachePath        string
	etaCachePath     string
	totalEtaSeconds  float64
	commandTimeCache map[string]time.Duration
}

type OptionsConfig struct {
	StopOnFailure        bool   `yaml:"stop-on-failure"`
	ShowStepSummary      bool   `yaml:"show-summary-steps"`
	ShowErrorSummary     bool   `yaml:"show-summary-errors"`
	ShowSummaryFooter    bool   `yaml:"show-summary-footer"`
	ShowFailureReport    bool   `yaml:"show-failure-summary"`
	LogPath              string `yaml:"log-path"`
	Vintage              bool   `yaml:"vintage"`
	MaxParallelCmds      int    `yaml:"max-parallel-commands"`
	ReplicaReplaceString string `yaml:"replica-replace-pattern"`
	ShowTaskEta          bool   `yaml:"show-task-times"`
	ShowTaskOutput       bool   `yaml:"show-task-output"`
	ShowSummaryTimes     bool   `yaml:"show-summary-times"`
	CollapseOnCompletion bool   `yaml:"collapse-on-completion"`
}

func defaultOptions() OptionsConfig {
	var defaultValues OptionsConfig
	defaultValues.StopOnFailure = true
	defaultValues.ShowSummaryFooter = true
	defaultValues.ShowErrorSummary = true
	defaultValues.ShowStepSummary = true
	defaultValues.ShowTaskOutput = true
	defaultValues.ShowFailureReport = true
	defaultValues.ReplicaReplaceString = "?"
	defaultValues.MaxParallelCmds = 4
	defaultValues.ShowSummaryTimes = true
	defaultValues.ShowTaskEta = true
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

func MinMax(array []float64) (float64, float64) {
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
	return min, max
}

func remove(slice []float64, value float64) []float64 {
	for index, arrValue := range slice {
		if arrValue == value {
			return append(slice[:index], slice[index+1:]...)
		}
	}
	return slice
}

func readConfig() {
	// fetch the ETA cache from disk (this must be done before fetching/parsing the run.yaml)...
	cwd, err := os.Getwd()
	CheckError(err, "Unable to get CWD.")

	config.cachePath = path.Join(cwd, ".bashful")
	config.logCachePath = path.Join(config.cachePath, "logs")
	config.etaCachePath = path.Join(config.cachePath, "eta")

	config.commandTimeCache = make(map[string]time.Duration)
	if doesFileExist(config.etaCachePath) {
		err := Load(config.etaCachePath, &config.commandTimeCache)
		CheckError(err, "Unable to load command eta cache.")
	}

	// fetch and parse the run.yaml user file...
	config.Options = defaultOptions()

	yamlString, err := ioutil.ReadFile(os.Args[1])
	CheckError(err, "Unable to read yaml config.")

	err = yaml.Unmarshal(yamlString, &config)
	CheckError(err, "Unable to parse yaml config.")

	var finalTasks []Task

	// initialize tasks with default values...
	for index := range config.Tasks {
		task := &config.Tasks[index]
		// finalize task by appending to the set of final tasks
		if len(task.ForEach) > 0 {
			taskName, taskCmdString := task.Name, task.CmdString
			for _, replicaValue := range task.ForEach {
				task.Name = taskName
				task.CmdString = taskCmdString
				task.Create(0, replicaValue)
				finalTasks = append(finalTasks, *task)
			}
		} else {
			task.Create(0, "")
			finalTasks = append(finalTasks, *task)
		}
	}

	// now that all tasks have been inflated, set the total eta
	for index := range finalTasks {
		task := &finalTasks[index]
		config.totalEtaSeconds += task.EstimatedRuntime()
	}

	// replace the current config with the inflated list of final tasks
	config.Tasks = finalTasks
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
