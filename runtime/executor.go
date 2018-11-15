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

func newExecutor(tasks []*Task) *Executor {
	executor := &Executor{
		Environment:    make(map[string]string, 0),
		FailedTasks:    make([]*Task, 0),
		Tasks:          tasks,
		CompletedTasks: make([]*Task, 0),
		eventHandlers:  make([]EventHandler, 0),
	}

	// todo: assigning to the Executor plan should be somewhere else
	for _, task := range tasks {
		task.Executor = executor
		if task.Config.CmdString != "" || task.Config.URL != "" {
			executor.TotalTasks++
		}

		for _, subTask := range task.Children {
			subTask.Executor = executor
			if subTask.Config.CmdString != "" || subTask.Config.URL != "" {
				executor.TotalTasks++
			}
		}

	}

	return executor
}

func (executor *Executor) addEventHandler(handler EventHandler) {
	executor.eventHandlers = append(executor.eventHandlers, handler)
}

func (executor *Executor) execute(task *Task) error {
	task.Run(executor.Environment)
	return nil
}

func (executor *Executor) run() error {
	for _, task := range executor.Tasks {
		executor.execute(task)

		if exitSignaled {
			break
		}
	}

	return nil
}
