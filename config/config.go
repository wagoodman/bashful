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
	"encoding/gob"
	"fmt"
	"os"
	"path"
	"regexp"
	"strings"
	"time"

	"github.com/deckarep/golang-set"
	"github.com/spf13/afero"
	"gopkg.in/yaml.v2"
	"github.com/wagoodman/bashful/utils"
)

// Config represents a superset of options parsed from the user yaml file (or derived from user values)
var Config struct {
	Cli CliOptions

	// Options is a global set of values to be applied to all tasks
	Options OptionsConfig `yaml:"config"`

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

	// CommandTimeCache is the task CmdString-to-ETASeconds for any previously run command (read from EtaCachePath)
	CommandTimeCache map[string]time.Duration
}

// readTimeCache fetches and reads a cache file from disk containing CmdString-to-ETASeconds. Note: this this must be done before fetching/parsing the run.yaml
func readTimeCache() {
	if Config.CachePath == "" {
		cwd, err := os.Getwd()
		utils.CheckError(err, "Unable to get CWD.")
		Config.CachePath = path.Join(cwd, ".bashful")
	}

	Config.DownloadCachePath = path.Join(Config.CachePath, "downloads")
	Config.LogCachePath = path.Join(Config.CachePath, "logs")
	Config.EtaCachePath = path.Join(Config.CachePath, "eta")

	// create the cache dirs if they do not already exist
	if _, err := os.Stat(Config.CachePath); os.IsNotExist(err) {
		os.Mkdir(Config.CachePath, 0755)
	}
	if _, err := os.Stat(Config.DownloadCachePath); os.IsNotExist(err) {
		os.Mkdir(Config.DownloadCachePath, 0755)
	}
	if _, err := os.Stat(Config.LogCachePath); os.IsNotExist(err) {
		os.Mkdir(Config.LogCachePath, 0755)
	}

	Config.CommandTimeCache = make(map[string]time.Duration)
	if utils.DoesFileExist(Config.EtaCachePath) {
		err := Load(Config.EtaCachePath, &Config.CommandTimeCache)
		utils.CheckError(err, "Unable to load command eta cache.")
	}
}

// replaceArguments replaces the command line arguments in the given string
func replaceArguments(source string) string {
	replaced := source
	for i, arg := range Config.Cli.Args {
		replaced = strings.Replace(replaced, fmt.Sprintf("$%v", i+1), arg, -1)
	}
	replaced = strings.Replace(replaced, "$*", strings.Join(Config.Cli.Args, " "), -1)
	return replaced
}

type includeMatch struct {
	includeFile string
	startIdx    int
	endIdx      int
}

func getIndentSize(yamlString []byte, startIdx int) int {
	spaces := 0
	for idx := startIdx; idx > 0; idx++ {
		char := yamlString[idx]
		if char == '\n' {
			spaces = 0
		} else if char == ' ' {
			spaces++
		} else {
			break
		}
	}
	return spaces
}

func indentBytes(b []byte, size int) []byte {
	prefix := []byte(strings.Repeat(" ", size))
	var res []byte
	bol := true
	for _, c := range b {
		if bol && c != '\n' {
			res = append(res, prefix...)
		}
		res = append(res, c)
		bol = c == '\n'
	}
	return res
}

func assembleIncludes(yamlString []byte) []byte {
	appFs := afero.NewOsFs()
	listInc := regexp.MustCompile(`(?m:\s*-\s\$include\s+(?P<filename>.+)$)`)
	mapInc := regexp.MustCompile(`(?m:^\s*\$include:\s+(?P<filename>.+)$)`)

	for _, pattern := range []*regexp.Regexp{listInc, mapInc} {
		for ok := true; ok; {
			indexes := pattern.FindSubmatchIndex(yamlString)
			ok = len(indexes) != 0
			if ok {
				match := includeMatch{
					includeFile: string(yamlString[indexes[2]:indexes[3]]),
					startIdx:    indexes[0],
					endIdx:      indexes[1],
				}

				indent := getIndentSize(yamlString, match.startIdx)

				contents, err := afero.ReadFile(appFs, match.includeFile)
				utils.CheckError(err, "Unable to read file: "+match.includeFile)
				indentedContents := indentBytes(contents, indent)
				result := []byte{}
				result = append(result, yamlString[:match.startIdx]...)
				result = append(result, '\n')
				result = append(result, indentedContents...)
				result = append(result, yamlString[match.endIdx:]...)
				yamlString = result
			}
		}
	}

	return yamlString
}

// readRunYaml fetches and reads the user given yaml file from disk and populates the global Config object
func ParseRunYaml(yamlString []byte) {
	// fetch and parse the run.yaml user file...
	Config.Options = NewOptionsConfig()

	yamlString = assembleIncludes(yamlString)
	err := yaml.Unmarshal(yamlString, &Config)
	utils.CheckError(err, "Error: Unable to parse given yaml")

	Config.Options.validate()

	// duplicate tasks with for-each clauses
	for i := 0; i < len(Config.TaskConfigs); i++ {
		taskConfig := &Config.TaskConfigs[i]
		newTaskConfigs := taskConfig.compile()
		if len(newTaskConfigs) > 0 {
			for _, newConfig := range newTaskConfigs {
				// insert the copy after current index
				Config.TaskConfigs = append(Config.TaskConfigs[:i], append([]TaskConfig{newConfig}, Config.TaskConfigs[i:]...)...)
				i++
			}
			// remove current index
			Config.TaskConfigs = append(Config.TaskConfigs[:i], Config.TaskConfigs[i+1:]...)
			i--
		}

		for j := 0; j < len(taskConfig.ParallelTasks); j++ {
			subTaskConfig := &taskConfig.ParallelTasks[j]
			newSubTaskConfigs := subTaskConfig.compile()

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
	for index := range Config.TaskConfigs {
		taskConfig := &Config.TaskConfigs[index]
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
	if len(Config.Cli.RunTags) > 0 {
		for i := 0; i < len(Config.TaskConfigs); i++ {
			taskConfig := &Config.TaskConfigs[i]
			subTasksWithActiveTag := false

			for j := 0; j < len(taskConfig.ParallelTasks); j++ {
				subTaskConfig := &taskConfig.ParallelTasks[j]
				matchedTaskTags := Config.Cli.RunTagSet.Intersect(subTaskConfig.TagSet)
				if len(matchedTaskTags.ToSlice()) > 0 || (len(subTaskConfig.Tags) == 0 && !Config.Cli.ExecuteOnlyMatchedTags) {
					subTasksWithActiveTag = true
					continue
				}
				// this particular subtask does not have a matching tag: prune this task
				taskConfig.ParallelTasks = append(taskConfig.ParallelTasks[:j], taskConfig.ParallelTasks[j+1:]...)
				j--
			}

			matchedTaskTags := Config.Cli.RunTagSet.Intersect(taskConfig.TagSet)
			if !subTasksWithActiveTag && len(matchedTaskTags.ToSlice()) == 0 && (len(taskConfig.Tags) > 0 || Config.Cli.ExecuteOnlyMatchedTags) {
				// this task does not have matching tags and there are no children with matching tags: prune this task
				Config.TaskConfigs = append(Config.TaskConfigs[:i], Config.TaskConfigs[i+1:]...)
				i--
			}
		}
	}
}

// ParseConfig is the entrypoint for all Config fetching and parsing. This returns a list of Task runtime objects to execute.
func ParseConfig(yamlString []byte) {
	Config.Cli.RunTagSet = mapset.NewSet()
	for _, tag := range Config.Cli.RunTags {
		Config.Cli.RunTagSet.Add(tag)
	}

	readTimeCache()

	ParseRunYaml(yamlString)

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
