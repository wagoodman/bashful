package config

import (
	"strings"
	"github.com/wagoodman/bashful/utils"
)

// NewTaskConfig creates a new TaskConfig populated with sane default values (derived from the global OptionsConfig)
func NewTaskConfig() (obj TaskConfig) {
	obj.IgnoreFailure = Config.Options.IgnoreFailure
	obj.StopOnFailure = Config.Options.StopOnFailure
	obj.ShowTaskOutput = Config.Options.ShowTaskOutput
	obj.EventDriven = Config.Options.EventDriven
	obj.CollapseOnCompletion = Config.Options.CollapseOnCompletion

	return obj
}

// UnmarshalYAML parses and creates a TaskConfig from a given user yaml string
func (taskConfig *TaskConfig) UnmarshalYAML(unmarshal func(interface{}) error) error {
	type defaults TaskConfig
	defaultValues := defaults(NewTaskConfig())

	if err := unmarshal(&defaultValues); err != nil {
		return err
	}

	*taskConfig = TaskConfig(defaultValues)

	if Config.Options.SingleLineDisplay {
		taskConfig.ShowTaskOutput = false
		taskConfig.CollapseOnCompletion = false
	}

	return nil
}

// allow passing a single value or multiple values into a yaml string (e.g. `tags: thing` or `{tags: [thing1, thing2]}`)
func (a *stringArray) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var multi []string
	err := unmarshal(&multi)
	if err != nil {
		var single string
		err := unmarshal(&single)
		if err != nil {
			return err
		}
		*a = []string{single}
	} else {
		*a = multi
	}
	return nil
}

func (taskConfig *TaskConfig) compile() (tasks []TaskConfig) {
	taskConfig.CmdString = replaceArguments(taskConfig.CmdString)
	if taskConfig.Name == "" {
		taskConfig.Name = taskConfig.CmdString
	} else {
		taskConfig.Name = replaceArguments(taskConfig.Name)
	}

	if len(taskConfig.ForEach) > 0 {
		for _, replicaValue := range taskConfig.ForEach {
			// make replacements of select attributes on a copy of the Config
			newConfig := *taskConfig

			// ensure we don't re-compile a replica with more replica's of itself
			newConfig.ForEach = make([]string, 0)

			if newConfig.Name == "" {
				newConfig.Name = newConfig.CmdString
			}
			newConfig.Name = strings.Replace(newConfig.Name, Config.Options.ReplicaReplaceString, replicaValue, -1)
			newConfig.CmdString = strings.Replace(newConfig.CmdString, Config.Options.ReplicaReplaceString, replicaValue, -1)
			newConfig.URL = strings.Replace(newConfig.URL, Config.Options.ReplicaReplaceString, replicaValue, -1)

			newConfig.Tags = make(stringArray, len(taskConfig.Tags))
			for k := range taskConfig.Tags {
				newConfig.Tags[k] = strings.Replace(taskConfig.Tags[k], Config.Options.ReplicaReplaceString, replicaValue, -1)
			}

			// insert the copy after current index
			tasks = append(tasks, newConfig)
		}
	}
	return tasks
}

func (taskConfig *TaskConfig) validate() {
	if taskConfig.CmdString == "" && len(taskConfig.ParallelTasks) == 0 && taskConfig.URL == "" {
		utils.ExitWithErrorMessage("Task '" + taskConfig.Name + "' misconfigured (A configured task must have at least 'cmd', 'url', or 'parallel-tasks' configured)")
	}
}

