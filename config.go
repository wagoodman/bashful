package main

import (
	"encoding/gob"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"os"
	"path"
	"runtime"

	yaml "gopkg.in/yaml.v2"
)

type OptionsConfig struct {
	StopOnFailure        bool   `yaml:"stop-on-failure"`
	ShowSteps            bool   `yaml:"show-steps"`
	ShowSummaryFooter    bool   `yaml:"show-summary-footer"`
	ShowFailureReport    bool   `yaml:"show-failure-summary"`
	LogPath              string `yaml:"log-path"`
	Vintage              bool   `yaml:"vintage"`
	MaxParallelCmds      int    `yaml:"max-parallel-commands"`
	ReplicaReplaceString string `yaml:"replica-replace-pattern"`
	ShowTaskEta          bool   `yaml:"show-task-eta"`
	ShowSummaryTimes     bool   `yaml:"show-summary-times"`
}

type RunConfig struct {
	Options OptionsConfig `yaml:"config"`
	Tasks   []Task        `yaml:"tasks"`
}

func defaultOptions() OptionsConfig {
	var defaultValues OptionsConfig
	defaultValues.StopOnFailure = true
	defaultValues.ShowSteps = false
	defaultValues.ShowSummaryFooter = true
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
	return nil
}

func (conf *RunConfig) read() {
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Println("Unable to get CWD!")
		fmt.Println(err)
		os.Exit(1)
	}

	CachePath = path.Join(cwd, ".bashful")
	LogCachePath = path.Join(CachePath, "logs")
	EtaCachePath = path.Join(CachePath, "eta")

	// note: you must load the eta cache before the run.yml file
	if Exists(EtaCachePath) {
		err := Load(EtaCachePath, &CommandTimeCache)
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
		// finalize task by appending to the set of final tasks
		if task.CmdString != "" && task.Command.EstimatedRuntime != -1 {
			TotalEtaSeconds += task.Command.EstimatedRuntime.Seconds()
		}

		var maxParallelEstimatedRuntime float64
		for subIndex := range task.ParallelTasks {
			subTask := &task.ParallelTasks[subIndex]
			if subTask.CmdString != "" && subTask.Command.EstimatedRuntime != -1 {
				maxParallelEstimatedRuntime = math.Max(maxParallelEstimatedRuntime, subTask.Command.EstimatedRuntime.Seconds())
			}
		}
		TotalEtaSeconds += maxParallelEstimatedRuntime
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
