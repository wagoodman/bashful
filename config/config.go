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
	"fmt"
	"github.com/deckarep/golang-set"
	"github.com/spf13/afero"
	"github.com/wagoodman/bashful/utils"
	"gopkg.in/yaml.v2"
	"os"
	"path"
	"strings"
)

var globalOptions *Options

func NewConfig(yamlString []byte, options *Cli) (*Config, error) {
	config := Config{}
	if options != nil {
		config.Cli = *options
	}

	if config.CachePath == "" {
		cwd, err := os.Getwd()
		utils.CheckError(err, "Unable to get CWD.")
		config.CachePath = path.Join(cwd, ".bashful")
	}

	config.DownloadCachePath = path.Join(config.CachePath, "downloads")
	config.LogCachePath = path.Join(config.CachePath, "logs")
	config.EtaCachePath = path.Join(config.CachePath, "eta")

	err := config.compile(yamlString)
	return &config, err
}

func (config *Config) validate() error {
	var err error
	// ensure not too many nestings of parallel tasks has been configured
	for _, taskConfig := range config.TaskConfigs {
		for _, subTaskConfig := range taskConfig.ParallelTasks {
			if len(subTaskConfig.ParallelTasks) > 0 {
				return fmt.Errorf("nested parallel tasks not allowed (violated by name:'" + subTaskConfig.Name + "' cmd:'" + subTaskConfig.CmdString + "')")
			}
			err = subTaskConfig.validate()
			if err != nil {
				return err
			}

		}
		err = taskConfig.validate()
		if err != nil {
			return err
		}
	}
	return err
}

// replaceArguments replaces the command line arguments in the given string
func (config *Config) replaceArguments(source string) string {
	replaced := source
	for i, arg := range config.Cli.Args {
		replaced = strings.Replace(replaced, fmt.Sprintf("$%v", i+1), arg, -1)
	}
	replaced = strings.Replace(replaced, "$*", strings.Join(config.Cli.Args, " "), -1)
	return replaced
}

// compile parses the given user yaml and populates the config object based on the cli arguments
func (config *Config) compile(yamlString []byte) error {
	var err error

	// setup default options used when unmarshalling the config
	globalOptions = NewOptions()
	config.Options = *globalOptions

	// assemble the config from multiple files (if necessary)
	configAssembler := newAssembler(afero.NewOsFs())
	yamlString = configAssembler.assemble(yamlString)

	err = yaml.Unmarshal(yamlString, &config)
	if err != nil {
		return fmt.Errorf("unable to parse yaml: %v", err)
	}

	err = config.validate()
	if err != nil {
		return fmt.Errorf("yaml invalid: %v", err)
	}

	// duplicate tasks with for-each clauses
	for i := 0; i < len(config.TaskConfigs); i++ {
		taskConfig := &config.TaskConfigs[i]
		newTaskConfigs := taskConfig.compile(config)
		if len(newTaskConfigs) > 0 {
			for _, newConfig := range newTaskConfigs {
				// insert the copy after current index
				config.TaskConfigs = append(config.TaskConfigs[:i], append([]TaskConfig{newConfig}, config.TaskConfigs[i:]...)...)
				i++
			}
			// remove current index
			config.TaskConfigs = append(config.TaskConfigs[:i], config.TaskConfigs[i+1:]...)
			i--
		}

		for j := 0; j < len(taskConfig.ParallelTasks); j++ {
			subTaskConfig := &taskConfig.ParallelTasks[j]
			newSubTaskConfigs := subTaskConfig.compile(config)

			if len(newSubTaskConfigs) > 0 {
				// remove the index with the template taskConfig
				taskConfig.ParallelTasks = append(taskConfig.ParallelTasks[:j], taskConfig.ParallelTasks[j+1:]...)
				for _, newConfig := range newSubTaskConfigs {
					// insert the copy after current index
					taskConfig.ParallelTasks = append(taskConfig.ParallelTasks[:j], append([]TaskConfig{newConfig}, taskConfig.ParallelTasks[j:]...)...)
					j++
				}
			}
		}
	}

	// child tasks should inherit parent Config tags
	for index := range config.TaskConfigs {
		taskConfig := &config.TaskConfigs[index]
		taskConfig.TagSet = mapset.NewSet()
		for _, tag := range taskConfig.Tags {
			taskConfig.TagSet.Add(tag)
		}
		for subIndex := range taskConfig.ParallelTasks {
			subTaskConfig := &taskConfig.ParallelTasks[subIndex]
			subTaskConfig.Tags = append(subTaskConfig.Tags, taskConfig.Tags...)
			subTaskConfig.TagSet = mapset.NewSet()
			for _, tag := range subTaskConfig.Tags {
				subTaskConfig.TagSet.Add(tag)
			}
		}
	}

	// prune the set of tasks that will not run given the set of cli options
	if len(config.Cli.RunTags) > 0 {
		for i := 0; i < len(config.TaskConfigs); i++ {
			taskConfig := &config.TaskConfigs[i]
			subTasksWithActiveTag := false

			for j := 0; j < len(taskConfig.ParallelTasks); j++ {
				subTaskConfig := &taskConfig.ParallelTasks[j]
				matchedTaskTags := config.Cli.RunTagSet.Intersect(subTaskConfig.TagSet)
				if len(matchedTaskTags.ToSlice()) > 0 || (len(subTaskConfig.Tags) == 0 && !config.Cli.ExecuteOnlyMatchedTags) {
					subTasksWithActiveTag = true
					continue
				}
				// this particular subtask does not have a matching tag: prune this task
				taskConfig.ParallelTasks = append(taskConfig.ParallelTasks[:j], taskConfig.ParallelTasks[j+1:]...)
				j--
			}

			matchedTaskTags := config.Cli.RunTagSet.Intersect(taskConfig.TagSet)
			if !subTasksWithActiveTag && len(matchedTaskTags.ToSlice()) == 0 && (len(taskConfig.Tags) > 0 || config.Cli.ExecuteOnlyMatchedTags) {
				// this task does not have matching tags and there are no children with matching tags: prune this task
				config.TaskConfigs = append(config.TaskConfigs[:i], config.TaskConfigs[i+1:]...)
				i--
			}
		}
	}
	return nil
}
