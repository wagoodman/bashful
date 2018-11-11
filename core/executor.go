package core

func newExecutor(tasks []*Task) *Executor {
	invoker := &Executor{
		environment: make(map[string]string, 0),
		FailedTasks: make([]*Task, 0),
		Tasks:       tasks,
		CompletedTasks: make([]*Task, 0),
	}

	// todo: assigning to the Executor plan should be somewhere else
	for _, task := range tasks {
		task.invoker = invoker
		if task.Config.CmdString != "" || task.Config.URL != "" {
			invoker.TotalTasks++
		}

		for _, subTask := range task.Children {
			subTask.invoker = invoker
			if subTask.Config.CmdString != "" || subTask.Config.URL != "" {
				invoker.TotalTasks++
			}
		}

	}

	return invoker
}

func (executor *Executor) execute(task *Task) error {
	task.Run(executor.environment)
	return nil
}

func (executor *Executor) run() error {
	for _, task := range executor.Tasks {
		executor.execute(task)

		if ExitSignaled {
			break
		}
	}

	return nil
}
