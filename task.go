package main

import (
	"bufio"
	"bytes"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"math"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/lunixbochs/vtclean"
	color "github.com/mgutz/ansi"
	"github.com/tj/go-spin"
	terminal "github.com/wayneashleyberry/terminal-dimensions"
)

var (
	ticker         *time.Ticker
	spinner        *spin.Spinner = spin.New()
	nextDisplayIdx               = 0
)

var TaskStats struct {
	runningCmds      int
	completedTasks   int
	totalFailedTasks int
	totalTasks       int
}


type Task struct {
	Config               TaskConfig
	Display              TaskDisplay
	Command              TaskCommand
	LogChan              chan LogItem
	LogFile              *os.File
	ErrorBuffer          *bytes.Buffer
	Children             []*Task
	lastStartedTask int

	resultChan      chan CmdIR
	waiter          sync.WaitGroup
	status          CommandStatus
	failedTasks     []*Task
}

type TaskDisplay struct {
	Template *template.Template
	Index    int
	Values   LineInfo
}

type TaskCommand struct {
	Cmd              *exec.Cmd
	StartTime        time.Time
	StopTime         time.Time
	EstimatedRuntime time.Duration
	Started          bool
	Complete         bool
	ReturnCode       int
}

type CommandStatus int32

const (
	StatusRunning CommandStatus = iota
	StatusPending
	StatusSuccess
	StatusError
)

func (status CommandStatus) Color(attributes string) string {
	switch status {
	case StatusRunning:
		return color.ColorCode("22+" + attributes) //28

	case StatusPending:
		return color.ColorCode("22+" + attributes)

	case StatusSuccess:
		return color.ColorCode("green+h" + attributes)

	case StatusError:
		return color.ColorCode("red+h" + attributes)

	}
	return "INVALID COMMAND STATUS"
}

type CmdIR struct {
	Task       *Task
	Status     CommandStatus
	Stdout     string
	Stderr     string
	Complete   bool
	ReturnCode int
}

type LineInfo struct {
	Status  string
	Title   string
	Msg     string
	Spinner string
	Eta     string
	Split   string
}


func NewTask(taskConfig TaskConfig, displayStartIdx int, replicaValue string) Task {
	task := Task{Config: taskConfig}
	task.inflate(displayStartIdx, replicaValue)

	for subIndex := range taskConfig.ParallelTasks {
		subTaskConfig := &taskConfig.ParallelTasks[subIndex]

		if len(subTaskConfig.ForEach) > 0 {
			subTaskName, subTaskCmdString := subTaskConfig.Name, subTaskConfig.CmdString
			for _, subReplicaValue := range subTaskConfig.ForEach {
				subTaskConfig.Name = subTaskName
				subTaskConfig.CmdString = subTaskCmdString
				subTask := NewTask(*subTaskConfig, nextDisplayIdx, subReplicaValue)
				subTask.Display.Template = lineParallelTemplate

				task.Children = append(task.Children, &subTask)
				nextDisplayIdx++
			}
		} else {
			subTask := NewTask(*subTaskConfig, nextDisplayIdx, replicaValue)
			subTask.Display.Template = lineParallelTemplate

			task.Children = append(task.Children, &subTask)
			nextDisplayIdx++
		}
	}

	if len(task.Children) > 1 {
		task.Children[len(task.Children)-1].Display.Template = lineLastParallelTemplate
	}
	return task
}

func (task *Task) inflate(displayIdx int, replicaValue string) {
	cmdString := task.Config.CmdString
	name := task.Config.Name

	if cmdString != "" {
		TaskStats.totalTasks++
	}

	if cmdString == "" && len(task.Config.ParallelTasks) == 0 {
		exitWithErrorMessage("Task '" + name + "' misconfigured (A configured task must have at least either 'cmd' or 'parallel-tasks' configured)")
	}

	if replicaValue != "" {
		cmdString = strings.Replace(cmdString, config.Options.ReplicaReplaceString, replicaValue, -1)
	}

	task.Config.CmdString = cmdString

	if eta, ok := config.commandTimeCache[task.Config.CmdString]; ok {
		task.Command.EstimatedRuntime = eta
	} else {
		task.Command.EstimatedRuntime = time.Duration(-1)
	}

	//command := strings.Split(cmdString, " ")
	// task.Command.Cmd = exec.Command(command[0], command[1:]...)
	task.Command.Cmd = exec.Command(os.Getenv("SHELL"), "-c", fmt.Sprintf("\"%q\"", cmdString))

	// set this command as a process group
	task.Command.Cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	task.Command.ReturnCode = -1
	task.Display.Template = lineDefaultTemplate
	task.Display.Index = displayIdx
	task.ErrorBuffer = bytes.NewBufferString("")



	task.resultChan= make(chan CmdIR)
	task.status=     StatusPending



	// set the name
	if name == "" {
		task.Config.Name = cmdString
	} else {
		if replicaValue != "" {
			name = strings.Replace(name, config.Options.ReplicaReplaceString, replicaValue, -1)
		}
		task.Config.Name = name
	}

}

func (task *Task) Kill() {
	if task.Config.CmdString != "" && task.Command.Started && !task.Command.Complete {
		syscall.Kill(-task.Command.Cmd.Process.Pid, syscall.SIGKILL)
	}

	for _, subTask := range task.Children {
		if subTask.Config.CmdString != "" && subTask.Command.Started && !subTask.Command.Complete {
			syscall.Kill(-subTask.Command.Cmd.Process.Pid, syscall.SIGKILL)
		}
	}

}

//func (task *Task) Tasks() (tasks []*Task) {
//	if task.Config.CmdString != "" {
//		tasks = append(tasks, task)
//	} else {
//		for nestIdx := range task.Children {
//			tasks = append(tasks, task.Children[nestIdx])
//		}
//	}
//	return tasks
//}

func (task *Task) String(terminalWidth int) string {

	if task.Command.Complete {
		task.Display.Values.Eta = ""
		if task.Command.ReturnCode != 0 && task.Config.IgnoreFailure == false {
			task.Display.Values.Msg = red("Exited with error (" + strconv.Itoa(task.Command.ReturnCode) + ")")
		}
	}

	// set the name
	if task.Config.Name == "" {
		if len(task.Config.CmdString) > 25 {
			task.Config.Name = task.Config.CmdString[:22] + "..."
		} else {
			task.Config.Name = task.Config.CmdString
		}
	}

	// display
	var message bytes.Buffer

	// get a string with the summary line without a split gap or message
	task.Display.Values.Split = ""
	originalMessage := task.Display.Values.Msg
	task.Display.Values.Msg = ""
	task.Display.Template.Execute(&message, task.Display.Values)

	// calculate the max width of the message and trim it
	maxMessageWidth := terminalWidth - visualLength(message.String())
	task.Display.Values.Msg = originalMessage
	if visualLength(task.Display.Values.Msg) > maxMessageWidth {
		task.Display.Values.Msg = trimToVisualLength(task.Display.Values.Msg, maxMessageWidth-3) + "..."
	}

	// calculate a space buffer to push the eta to the right
	message.Reset()
	task.Display.Template.Execute(&message, task.Display.Values)
	splitWidth := terminalWidth - visualLength(message.String())
	if splitWidth < 0 {
		splitWidth = 0
	}

	message.Reset()

	// override the current spinner to empty or a config.Options.BulletChar
	if (!task.Command.Started || task.Command.Complete) && len(task.Children) == 1 {
		task.Display.Values.Spinner = config.Options.BulletChar
	} else if task.Command.Complete {
		task.Display.Values.Spinner = ""
	}

	task.Display.Values.Split = strings.Repeat(" ", splitWidth)
	task.Display.Template.Execute(&message, task.Display.Values)

	return message.String()
}

func (task *Task) display() {
	terminalWidth, _ := terminal.Width()
	Screen().Display(task.String(int(terminalWidth)), task.Display.Index)
}

func (task *Task) EstimatedRuntime() float64 {
	var etaSeconds float64
	// finalize task by appending to the set of final tasks
	if task.Config.CmdString != "" && task.Command.EstimatedRuntime != -1 {
		etaSeconds += task.Command.EstimatedRuntime.Seconds()
	}

	var maxParallelEstimatedRuntime float64
	var taskEndSecond []float64
	var currentSecond float64
	var remainingParallelTasks = config.Options.MaxParallelCmds

	for subIndex := range task.Children {
		subTask := task.Children[subIndex]
		if subTask.Config.CmdString != "" && subTask.Command.EstimatedRuntime != -1 {
			// this is a sub task with an eta
			if remainingParallelTasks == 0 {

				// we've started all possible tasks, now they should stop...
				// select the first task to stop
				remainingParallelTasks++
				minEndSecond, _, err := MinMax(taskEndSecond)
				CheckError(err, "No min eta for empty array!")
				taskEndSecond = removeOneValue(taskEndSecond, minEndSecond)
				currentSecond = minEndSecond
			}

			// we are still starting tasks
			taskEndSecond = append(taskEndSecond, currentSecond+subTask.Command.EstimatedRuntime.Seconds())
			remainingParallelTasks--

			_, maxEndSecond, err := MinMax(taskEndSecond)
			CheckError(err, "No max eta for empty array!")
			maxParallelEstimatedRuntime = math.Max(maxParallelEstimatedRuntime, maxEndSecond)
		}

	}
	etaSeconds += maxParallelEstimatedRuntime
	return etaSeconds
}

func (task *Task) CurrentEta() string {
	var eta, etaValue string

	if config.Options.ShowTaskEta {
		running := time.Since(task.Command.StartTime)
		etaValue = "?"
		if task.Command.EstimatedRuntime > 0 {
			etaValue = showDuration(time.Duration(task.Command.EstimatedRuntime.Seconds()-running.Seconds()) * time.Second)
		}
		eta = fmt.Sprintf(bold("[%s]"), etaValue)
	}
	return eta
}

func variableSplitFunc(data []byte, atEOF bool) (advance int, token []byte, err error) {

	// Return nothing if at end of file and no data passed
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}

	// Case: \n
	if i := strings.Index(string(data), "\n"); i >= 0 {
		return i + 1, data[0:i], nil
	}

	// Case: \r
	if i := strings.Index(string(data), "\r"); i >= 0 {
		return i + 1, data[0:i], nil
	}

	// Case: it's just too long
	terminalWidth, _ := terminal.Width()
	if len(data) > int(terminalWidth*2) {
		return int(terminalWidth * 2), data[0:int(terminalWidth*2)], nil
	}

	// TODO: by some ansi escape sequences

	// If at end of file with data return the data
	if atEOF {
		return len(data), data, nil
	}

	return
}

func (task *Task) runSingleCmd(resultChan chan CmdIR, waiter *sync.WaitGroup) {
	logToMain("Started Task: "+task.Config.Name, INFO_FORMAT)

	task.Command.StartTime = time.Now()

	resultChan <- CmdIR{Task: task, Status: StatusRunning, ReturnCode: -1}
	waiter.Add(1)
	defer waiter.Done()

	tempFile, _ := ioutil.TempFile(config.logCachePath, "")
	task.LogFile = tempFile
	task.LogChan = make(chan LogItem)
	go SingleLogger(task.LogChan, task.Config.Name, tempFile.Name())

	stdoutPipe, _ := task.Command.Cmd.StdoutPipe()
	stderrPipe, _ := task.Command.Cmd.StderrPipe()

	task.Command.Cmd.Start()

	var readPipe func(chan string, io.ReadCloser)

	readPipe = func(resultChan chan string, pipe io.ReadCloser) {
		defer close(resultChan)

		scanner := bufio.NewScanner(pipe)
		scanner.Split(variableSplitFunc)
		for scanner.Scan() {
			message := scanner.Text()
			resultChan <- vtclean.Clean(message, false)
		}
	}

	stdoutChan := make(chan string, 1000)
	stderrChan := make(chan string, 1000)
	go readPipe(stdoutChan, stdoutPipe)
	go readPipe(stderrChan, stderrPipe)

	for {
		select {
		case stdoutMsg, ok := <-stdoutChan:
			if ok {
				// it seems that we are getting a bit behind... burn off elements without showing them on the screen
				if len(stdoutChan) > 100 {
					continue
				}

				if task.Config.EventDriven {
					// this is event driven... (signal this event)
					resultChan <- CmdIR{Task: task, Status: StatusRunning, Stdout: blue(stdoutMsg), ReturnCode: -1}
				} else {
					// on a polling interval... (do not create an event)
					task.Display.Values.Msg = blue(stdoutMsg)
				}
				task.LogChan <- LogItem{Name: task.Config.Name, Message: stdoutMsg + "\n"}

			} else {
				stdoutChan = nil
			}
		case stderrMsg, ok := <-stderrChan:
			if ok {

				if task.Config.EventDriven {
					// either this is event driven... (signal this event)
					resultChan <- CmdIR{Task: task, Status: StatusRunning, Stderr: red(stderrMsg), ReturnCode: -1}
				} else {
					// or on a polling interval... (do not create an event)
					task.Display.Values.Msg = red(stderrMsg)
				}
				task.LogChan <- LogItem{Name: task.Config.Name, Message: red(stderrMsg) + "\n"}
				task.ErrorBuffer.WriteString(stderrMsg + "\n")
			} else {
				stderrChan = nil
			}
		}
		if stdoutChan == nil && stderrChan == nil {
			break
		}
	}

	returnCode := 0
	returnCodeMsg := "unknown"
	if err := task.Command.Cmd.Wait(); err != nil {
		if exiterr, ok := err.(*exec.ExitError); ok {
			// The program has exited with an exit code != 0
			if status, ok := exiterr.Sys().(syscall.WaitStatus); ok {
				returnCode = status.ExitStatus()
			}
		} else {
			returnCode = -1
			returnCodeMsg = "Failed to run: " + err.Error()
			resultChan <- CmdIR{Task: task, Status: StatusError, Stderr: returnCodeMsg, ReturnCode: returnCode}
			task.LogChan <- LogItem{Name: task.Config.Name, Message: red(returnCodeMsg) + "\n"}
			task.ErrorBuffer.WriteString(returnCodeMsg + "\n")
		}
	}
	task.Command.StopTime = time.Now()

	logToMain("Completed Task: "+task.Config.Name+" (rc: "+returnCodeMsg+")", INFO_FORMAT)

	if returnCode == 0 || task.Config.IgnoreFailure {
		resultChan <- CmdIR{Task: task, Status: StatusSuccess, Complete: true, ReturnCode: returnCode}
	} else {
		resultChan <- CmdIR{Task: task, Status: StatusError, Complete: true, ReturnCode: returnCode}
		if task.Config.StopOnFailure {
			exitSignaled = true
		}
	}
}


func (task *Task) Pave() {
	logToMain(" Pave Task: "+task.Config.Name, MAJOR_FORMAT)
	var message bytes.Buffer
	hasParentCmd := task.Config.CmdString != ""
	hasHeader := len(task.Children) > 1
	numTasks := len(task.Children)
	if hasParentCmd {
		numTasks++
	}
	scr := Screen()
	scr.ResetFrame(numTasks, hasHeader, config.Options.ShowSummaryFooter)

	// make room for the title of a parallel proc group
	if hasHeader {
		message.Reset()
		lineObj := LineInfo{Status: StatusRunning.Color("i"), Title: task.Config.Name, Msg: "", Spinner: config.Options.BulletChar}
		task.Display.Template.Execute(&message, lineObj)
		scr.DisplayHeader(message.String())
	}

	if hasParentCmd {
		task.Display.Values = LineInfo{Status: StatusPending.Color("i"), Title: task.Config.Name}
		task.display()
	}

	for line := 0; line < len(task.Children); line++ {
		task.Children[line].Display.Values = LineInfo{Status: StatusPending.Color("i"), Title: task.Children[line].Config.Name}
		task.Children[line].display()
	}
}

func (task *Task) StartAvailableTasks() {
	if task.Config.CmdString != "" && !task.Command.Started && TaskStats.runningCmds < config.Options.MaxParallelCmds {
		go task.runSingleCmd(task.resultChan, &task.waiter)
		task.Command.Started = true
		TaskStats.runningCmds++
	}
	for ; TaskStats.runningCmds < config.Options.MaxParallelCmds && task.lastStartedTask < len(task.Children); task.lastStartedTask++ {
		go task.Children[task.lastStartedTask].runSingleCmd(task.resultChan, &task.waiter)
		task.Children[task.lastStartedTask].Command.Started = true
		TaskStats.runningCmds++
	}
}

func (task *Task) Completed(rc int) {
	task.Command.Complete = true
	task.Command.ReturnCode = rc
	close(task.LogChan)

	TaskStats.completedTasks++
	config.commandTimeCache[task.Config.CmdString] = task.Command.StopTime.Sub(task.Command.StartTime)
	TaskStats.runningCmds--
}

func (task *Task) listenAndDisplay() {
	scr := Screen()
	// just wait for stuff to come back
	for TaskStats.runningCmds > 0 {
		select {
		case <-ticker.C:
			spinner.Next()

			if task.Config.CmdString != "" {
				if !task.Command.Complete && task.Command.Started {
					task.Display.Values.Spinner = spinner.Current()
					task.Display.Values.Eta = task.CurrentEta()
				}
				task.display()
			}

			for _, taskObj := range task.Children {
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

		case msgObj := <-task.resultChan:
			eventTask := msgObj.Task

			// update the state before displaying...
			if msgObj.Complete {
				eventTask.Completed(msgObj.ReturnCode)
				task.StartAvailableTasks()
				task.status = msgObj.Status
				if msgObj.Status == StatusError {
					// update the group status to indicate a failed subtask
					TaskStats.totalFailedTasks++

					// keep note of the failed task for an after task report
					task.failedTasks = append(task.failedTasks, eventTask)
				}
			}

			if eventTask.Config.ShowTaskOutput == false {
				msgObj.Stderr = ""
				msgObj.Stdout = ""
			}

			if msgObj.Stderr != "" {
				eventTask.Display.Values = LineInfo{Status: msgObj.Status.Color("i"), Title: eventTask.Config.Name, Msg: msgObj.Stderr, Spinner: spinner.Current(), Eta: eventTask.CurrentEta()}
			} else {
				eventTask.Display.Values = LineInfo{Status: msgObj.Status.Color("i"), Title: eventTask.Config.Name, Msg: msgObj.Stdout, Spinner: spinner.Current(), Eta: eventTask.CurrentEta()}
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
		task.waiter.Wait()
	}

}

// TODO: this needs to be split off into more testable parts!
func (task *Task) Run() {

	var message bytes.Buffer

	scr := Screen()
	hasHeader := len(task.Children) > 1

	task.Pave()
	task.StartAvailableTasks()
	task.listenAndDisplay()

	// complete the proc group status
	if hasHeader {
		message.Reset()
		collapseSummary := ""
		if task.Config.CollapseOnCompletion && len(task.Children) > 1 {
			collapseSummary = purple(" (" + strconv.Itoa(len(task.Children)) + " tasks hidden)")
		}
		task.Display.Template.Execute(&message, LineInfo{Status: task.status.Color("i"), Title: task.Config.Name + collapseSummary, Spinner: config.Options.BulletChar})
		scr.DisplayHeader(message.String())
	}

	// collapse sections or parallel tasks...
	if task.Config.CollapseOnCompletion && len(task.Children) > 1 {

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