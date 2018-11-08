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

// CliOptions is the exhaustive set of all command line options available on bashful
type CliOptions struct {
	RunTags                []string
	RunTagSet              mapset.Set
	ExecuteOnlyMatchedTags bool
	Args                   []string
}

// OptionsConfig is the set of values to be applied to all tasks or affect general behavior
type OptionsConfig struct {
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

// NewOptionsConfig creates a new OptionsConfig populated with sane default values
func NewOptionsConfig() (obj OptionsConfig) {
	obj.BulletChar = "•"
	obj.CollapseOnCompletion = false
	obj.ColorError = 160
	obj.ColorPending = 22
	obj.ColorRunning = 22
	obj.ColorSuccess = 10
	obj.EventDriven = true
	obj.ExecReplaceString = "<exec>"
	obj.IgnoreFailure = false
	obj.MaxParallelCmds = 4
	obj.ReplicaReplaceString = "<replace>"
	obj.ShowFailureReport = true
	obj.ShowSummaryErrors = false
	obj.ShowSummaryFooter = true
	obj.ShowSummarySteps = true
	obj.ShowSummaryTimes = true
	obj.ShowTaskEta = false
	obj.ShowTaskOutput = true
	obj.StopOnFailure = true
	obj.SingleLineDisplay = false
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

	if options.SingleLineDisplay {
		options.ShowSummaryFooter = false
		options.CollapseOnCompletion = false
	}

	// the global options must be available when parsing the task yaml (does order matter?)
	Config.Options = *options
	return nil
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

type stringArray []string

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

func (taskConfig *TaskConfig) inflate() (tasks []TaskConfig) {
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

			// ensure we don't re-inflate a replica with more replica's of itself
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
func parseRunYaml(yamlString []byte) {
	// fetch and parse the run.yaml user file...
	Config.Options = NewOptionsConfig()

	yamlString = assembleIncludes(yamlString)
	err := yaml.Unmarshal(yamlString, &Config)
	utils.CheckError(err, "Error: Unable to parse given yaml")

	Config.Options.validate()

	// duplicate tasks with for-each clauses
	for i := 0; i < len(Config.TaskConfigs); i++ {
		taskConfig := &Config.TaskConfigs[i]
		newTaskConfigs := taskConfig.inflate()
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
			newSubTaskConfigs := subTaskConfig.inflate()

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

func (options *OptionsConfig) validate() {

	// ensure not too many nestings of parallel tasks has been configured
	for _, taskConfig := range Config.TaskConfigs {
		for _, subTaskConfig := range taskConfig.ParallelTasks {
			if len(subTaskConfig.ParallelTasks) > 0 {
				utils.ExitWithErrorMessage("Nested parallel tasks not allowed (violated by name:'" + subTaskConfig.Name + "' cmd:'" + subTaskConfig.CmdString + "')")
			}
			subTaskConfig.validate()
		}
		taskConfig.validate()
	}
}

func (taskConfig *TaskConfig) validate() {
	if taskConfig.CmdString == "" && len(taskConfig.ParallelTasks) == 0 && taskConfig.URL == "" {
		utils.ExitWithErrorMessage("Task '" + taskConfig.Name + "' misconfigured (A configured task must have at least 'cmd', 'url', or 'parallel-tasks' configured)")
	}
}

// ParseConfig is the entrypoint for all Config fetching and parsing. This returns a list of Task runtime objects to execute.
func ParseConfig(yamlString []byte) {
	Config.Cli.RunTagSet = mapset.NewSet()
	for _, tag := range Config.Cli.RunTags {
		Config.Cli.RunTagSet.Add(tag)
	}

	readTimeCache()

	parseRunYaml(yamlString)

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
