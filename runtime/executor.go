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

package runtime

import (
	"github.com/wagoodman/bashful/config"
	"github.com/wagoodman/bashful/utils"
	"os"
	"time"
)

func newExecutor(cfg *config.Config) *Executor {
	executor := &Executor{
		Environment:    make(map[string]string, 0),
		FailedTasks:    make([]*Task, 0),
		CompletedTasks: make([]*Task, 0),
		eventHandlers:  make([]EventHandler, 0),
		config:         cfg,
		cmdEtaCache:    make(map[string]time.Duration, 0),
		Tasks:          make([]*Task, 0),
	}

	for _, taskConfig := range cfg.TaskConfigs {
		// finalize task by appending to the set of final Tasks
		task := NewTask(taskConfig, executor, &cfg.Options)
		executor.Tasks = append(executor.Tasks, task)
	}

	return executor
}

// estimateRuntime fetches and reads a cache file from disk containing CmdString-to-ETASeconds. Note: this this must be done before fetching/parsing the run.yaml
func (executor *Executor) readEtaCache() {
	// create the cache dirs if they do not already exist
	if _, err := os.Stat(executor.config.CachePath); os.IsNotExist(err) {
		os.Mkdir(executor.config.CachePath, 0755)
	}

	// read the time cache
	executor.cmdEtaCache = make(map[string]time.Duration)
	if utils.DoesFileExist(executor.config.EtaCachePath) {
		err := utils.Load(executor.config.EtaCachePath, &executor.cmdEtaCache)
		utils.CheckError(err, "Unable to load command eta cache.")
	}

}

// estimateRuntime accumulates the ETA for all planned tasks
func (executor *Executor) estimateRuntime() {
	executor.readEtaCache()

	for _, task := range executor.Tasks {
		if task.Config.CmdString != "" || task.Config.URL != "" {
			executor.TotalTasks++
			if eta, ok := executor.cmdEtaCache[task.Config.CmdString]; ok {
				task.Command.addEstimatedRuntime(eta)
			}
		}

		for _, subTask := range task.Children {
			if subTask.Config.CmdString != "" || subTask.Config.URL != "" {
				executor.TotalTasks++
				if eta, ok := executor.cmdEtaCache[subTask.Config.CmdString]; ok {
					subTask.Command.addEstimatedRuntime(eta)
				}
			}
		}

		executor.config.TotalEtaSeconds += task.estimateRuntime()
	}
}

func (executor *Executor) addEventHandler(handler EventHandler) {
	executor.eventHandlers = append(executor.eventHandlers, handler)
}

func (executor *Executor) execute(task *Task) error {
	task.Execute(executor.Environment)
	return nil
}

func (executor *Executor) run() error {
	for _, task := range executor.Tasks {
		executor.execute(task)

		if exitSignaled {
			break
		}
	}
	for _, handler := range executor.eventHandlers {
		handler.Close()
	}

	err := utils.Save(executor.config.EtaCachePath, &executor.cmdEtaCache)
	utils.CheckError(err, "Unable to save command eta cache.")

	return nil
}
