package main

import (
	"io/ioutil"
	"log"
	"os"

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
	conf.Options = defaultOptions()

	// fmt.Println("Reading " + os.Args[1] + " ...")
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
}
