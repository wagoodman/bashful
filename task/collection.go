package task

import (
	"github.com/wagoodman/bashful/config"
	"time"
)

func Open() {

	StartTime = time.Now()
	if config.Config.Options.UpdateInterval > 150 {
		Ticker = time.NewTicker(time.Duration(config.Config.Options.UpdateInterval) * time.Millisecond)
	} else {
		Ticker = time.NewTicker(150 * time.Millisecond)
	}
	AllTasks = CreateTasks()
}

// CreateTasks is responsible for reading all parsed TaskConfigs and generating a list of Task runtime objects to later execute
func CreateTasks() (finalTasks []*Task) {

	// initialize tasks with default values
	for _, taskConfig := range config.Config.TaskConfigs {
		nextDisplayIdx = 0

		// finalize task by appending to the set of final tasks
		task := NewTask(taskConfig, nextDisplayIdx, "")
		finalTasks = append(finalTasks, task)
	}

	// now that all tasks have been inflated, set the total eta
	for _, task := range finalTasks {
		config.Config.TotalEtaSeconds += task.EstimateRuntime()
	}

	// replace the current Config with the inflated list of final tasks
	return finalTasks
}


func Close(failedTasks []*Task) {
	if config.Config.Options.ShowSummaryFooter {
		message := ""
		NewScreen().ResetFrame(0, false, true)
		if len(failedTasks) > 0 {
			if config.Config.Options.LogPath != "" {
				message = bold(" See log for details (" + config.Config.Options.LogPath + ")")
			}
			NewScreen().DisplayFooter(footer(StatusError, message))
		} else {
			NewScreen().DisplayFooter(footer(StatusSuccess, message))
		}
	}
}
