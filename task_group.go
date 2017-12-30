package main

import (
	"bytes"
	"strconv"
	"sync"
)

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
	status          CommandStatus
	failedTasks     []*Task
}

func NewTaskGroup(task *Task) TaskGroup {
	return TaskGroup{
		parentTask: task,
		tasks:      task.Tasks(),
		resultChan: make(chan CmdIR),
		status:     StatusPending,
	}
}

func (tg *TaskGroup) StartAvailableTasks() {
	for ; TaskStats.runningCmds < config.Options.MaxParallelCmds && tg.lastStartedTask < len(tg.tasks); tg.lastStartedTask++ {
		go tg.tasks[tg.lastStartedTask].runSingleCmd(tg.resultChan, &tg.waiter)
		tg.tasks[tg.lastStartedTask].Command.Started = true
		TaskStats.runningCmds++
	}
}

func (tg *TaskGroup) Completed(task *Task) {
	TaskStats.completedTasks++
	config.commandTimeCache[task.CmdString] = task.Command.StopTime.Sub(task.Command.StartTime)
	TaskStats.runningCmds--
	tg.StartAvailableTasks()
}

func (tg *TaskGroup) listenAndDisplay() {
	scr := Screen()
	// just wait for stuff to come back
	for TaskStats.runningCmds > 0 {
		select {
		case <-ticker.C:
			spinner.Next()

			for _, taskObj := range tg.tasks {
				if !taskObj.Command.Complete && taskObj.Command.Started {
					taskObj.Display.Values.Spinner = spinner.Current()
					taskObj.Display.Values.Eta = taskObj.CurrentEta()
				}
				taskObj.display()
			}

			// update the summary line
			if config.Options.ShowSummaryFooter {
				scr.DisplayFooter(footer(StatusPending, ""))
			}

		case msgObj := <-tg.resultChan:
			eventTask := msgObj.Task

			// update the state before displaying...
			if msgObj.Complete {
				eventTask.Completed(msgObj.ReturnCode)
				tg.Completed(eventTask)
				if msgObj.Status == StatusError {
					// update the group status to indicate a failed subtask
					tg.status = StatusError
					TaskStats.totalFailedTasks++

					// keep note of the failed task for an after task report
					tg.failedTasks = append(tg.failedTasks, eventTask)
				}
			}

			if eventTask.ShowTaskOutput == false {
				msgObj.Stderr = ""
				msgObj.Stdout = ""
			}

			if msgObj.Stderr != "" {
				eventTask.Display.Values = LineInfo{Status: msgObj.Status.Color("i"), Title: eventTask.Name, Msg: msgObj.Stderr, Spinner: spinner.Current(), Eta: eventTask.CurrentEta()}
			} else {
				eventTask.Display.Values = LineInfo{Status: msgObj.Status.Color("i"), Title: eventTask.Name, Msg: msgObj.Stdout, Spinner: spinner.Current(), Eta: eventTask.CurrentEta()}
			}

			eventTask.display()

			// update the summary line
			if config.Options.ShowSummaryFooter {
				scr.DisplayFooter(footer(StatusPending, ""))
			} else {
				scr.MovePastFrame(false)
			}

			if exitSignaled {
				break
			}

		}

	}

	if !exitSignaled {
		tg.waiter.Wait()
	}

}

// TODO: this needs to be split off into more testable parts!
func (tg *TaskGroup) Run() {

	var message bytes.Buffer

	scr := Screen()
	scr.Pave(tg)
	tg.StartAvailableTasks()
	hasHeader := len(tg.tasks) > 1

	tg.listenAndDisplay()

	// complete the proc group status
	if hasHeader {
		message.Reset()
		collapseSummary := ""
		if tg.parentTask.CollapseOnCompletion && len(tg.tasks) > 1 {
			collapseSummary = purple(" (" + strconv.Itoa(len(tg.tasks)) + " tasks hidden)")
		}
		tg.parentTask.Display.Template.Execute(&message, LineInfo{Status: tg.status.Color("i"), Title: tg.parentTask.Name + collapseSummary, Spinner: config.Options.BulletChar})
		scr.DisplayHeader(message.String())
	}

	// collapse sections or parallel tasks...
	if tg.parentTask.CollapseOnCompletion && len(tg.tasks) > 1 {

		// head to the top of the section (below the header) and erase all lines
		scr.EraseBelowHeader()

		// head back to the top of the section
		scr.MoveCursorToFirstLine()
	} else {
		// ... or this is a single task or configured not to collapse

		// instead, leave all of the text on the screen...
		// ...reset the cursor to the bottom of the section
		scr.MovePastFrame(false)
	}
}
