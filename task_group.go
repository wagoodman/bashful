package main

import "sync"

var TaskStats struct {
	runningCmds      int
	completedTasks   int
	totalFailedTasks int
	totalTasks       int
}

type TaskGroup struct {
	lastStartedTask int
	parentTask      *Task
	tasks           []*Task
	resultChan      chan CmdIR
	waiter          sync.WaitGroup
}

func NewTaskGroup(task *Task, tasks []*Task) TaskGroup {
	return TaskGroup{
		parentTask: task,
		tasks:      tasks,
		resultChan: make(chan CmdIR),
	}
}

func (taskGroup *TaskGroup) StartAvailableTasks() {
	for ; taskGroup.lastStartedTask < config.Options.MaxParallelCmds && taskGroup.lastStartedTask < len(taskGroup.tasks); taskGroup.lastStartedTask++ {
		go taskGroup.tasks[taskGroup.lastStartedTask].runSingleCmd(taskGroup.resultChan, &taskGroup.waiter)
		taskGroup.tasks[taskGroup.lastStartedTask].Command.Started = true
		TaskStats.runningCmds++
	}
}

func (taskGroup *TaskGroup) Completed(task *Task) {
	TaskStats.completedTasks++

	config.commandTimeCache[task.CmdString] = task.Command.StopTime.Sub(task.Command.StartTime)

	TaskStats.runningCmds--
	// if a thread has freed up, start the next task (if there are any left)
	if taskGroup.lastStartedTask < len(taskGroup.tasks) {
		go taskGroup.tasks[taskGroup.lastStartedTask].runSingleCmd(taskGroup.resultChan, &taskGroup.waiter)
		taskGroup.tasks[taskGroup.lastStartedTask].Command.Started = true
		TaskStats.runningCmds++
		taskGroup.lastStartedTask++
	}
}
