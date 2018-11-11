package config

import (
	"github.com/wagoodman/bashful/utils"
)

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
