package main

import (
	"encoding/gob"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path"
	"runtime"

	yaml "gopkg.in/yaml.v2"
)

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

type RunConfig struct {
	Options OptionsConfig `yaml:"config"`
	Tasks   []Task        `yaml:"tasks"`
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

func (conf *OptionsConfig) UnmarshalYAML(unmarshal func(interface{}) error) error {
	type defaults OptionsConfig
	var defaultValues defaults
	defaultValues = defaults(defaultOptions())

	if err := unmarshal(&defaultValues); err != nil {
		return err
	}

	*conf = OptionsConfig(defaultValues)
	// the global options must be available when parsing the task yaml (does order matter?)
	Options = *conf
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

func (conf *RunConfig) read() {
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Println("Unable to get CWD!")
		fmt.Println(err)
		os.Exit(1)
	}

	cachePath = path.Join(cwd, ".bashful")
	logCachePath = path.Join(cachePath, "logs")
	etaCachePath = path.Join(cachePath, "eta")

	// note: you must load the eta cache before the run.yml file
	if doesFileExist(etaCachePath) {
		err := Load(etaCachePath, &commandTimeCache)
		Check(err)
	}

	conf.Options = defaultOptions()

	// load the run.yml file
	yamlString, err := ioutil.ReadFile(os.Args[1])
	if err != nil {
		log.Printf("yamlFile.Get err   #%v ", err)
	}

	err = yaml.Unmarshal(yamlString, conf)
	if err != nil {
		log.Fatalf("Unmarshal: %v", err)
	}

	// This needs to be done as soon as the options are parsed so that defailt task options can reference the global
	// Options = conf.Options
	var finalTasks []Task

	// initialize tasks with default values
	for index := range conf.Tasks {
		task := &conf.Tasks[index]
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

	// replace the current config with the inflated list of final tasks
	conf.Tasks = finalTasks

	// now that all tasks have been inflated, set the total eta
	for index := range conf.Tasks {
		task := &conf.Tasks[index]
		totalEtaSeconds += task.EstimatedRuntime()
	}

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

func Check(e error) {
	if e != nil {
		_, file, line, _ := runtime.Caller(1)
		fmt.Println(line, "\t", file, "\n", e)
		os.Exit(1)
	}
}
