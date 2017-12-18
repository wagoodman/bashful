package main

import (
	"bytes"
	"fmt"
	"html/template"
	"io/ioutil"
	"math"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	cmd "github.com/go-cmd/cmd"
	color "github.com/mgutz/ansi"
	"github.com/tj/go-spin"
	terminal "github.com/wayneashleyberry/terminal-dimensions"
)

var (
	ticker  *time.Ticker  = time.NewTicker(150 * time.Millisecond)
	spinner *spin.Spinner = spin.New()
)

type Task struct {
	Name           string `yaml:"name"`
	CmdString      string `yaml:"cmd"`
	Display        TaskDisplay
	Command        TaskCommand
	StopOnFailure  bool     `yaml:"stop-on-failure"`
	ShowTaskOutput bool     `yaml:"show-output"`
	IgnoreFailure  bool     `yaml:"ignore-failure"`
	ParallelTasks  []Task   `yaml:"parallel-tasks"`
	ForEach        []string `yaml:"for-each"`
	LogChan        chan LogItem
	LogFile        *os.File
	ErrorBuffer    *bytes.Buffer
}

type TaskDisplay struct {
	Template *template.Template
	Index    int
	Values   LineInfo
}

type TaskCommand struct {
	Cmd              *cmd.Cmd
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

func (task *Task) UnmarshalYAML(unmarshal func(interface{}) error) error {
	type defaults Task
	var defaultValues defaults
	defaultValues.StopOnFailure = config.Options.StopOnFailure
	defaultValues.ShowTaskOutput = config.Options.ShowTaskOutput

	if err := unmarshal(&defaultValues); err != nil {
		return err
	}

	*task = Task(defaultValues)
	return nil
}

func (task *Task) Create(displayStartIdx int, replicaValue string) {
	task.inflate(displayStartIdx, replicaValue)

	if task.CmdString != "" {
		totalTasks++
	}

	var finalTasks []Task

	for subIndex := range task.ParallelTasks {
		subTask := &task.ParallelTasks[subIndex]

		if len(subTask.ForEach) > 0 {
			subTaskName, subTaskCmdString := subTask.Name, subTask.CmdString
			for subReplicaIndex, subReplicaValue := range subTask.ForEach {
				subTask.Name = subTaskName
				subTask.CmdString = subTaskCmdString
				subTask.Create(subReplicaIndex, subReplicaValue)

				if subReplicaIndex == len(subTask.ForEach)-1 {
					subTask.Display.Template = lineLastParallelTemplate
				} else {
					subTask.Display.Template = lineParallelTemplate
				}

				finalTasks = append(finalTasks, *subTask)
			}
		} else {
			subTask.inflate(subIndex, replicaValue)
			totalTasks++

			if subIndex == len(task.ParallelTasks)-1 {
				subTask.Display.Template = lineLastParallelTemplate
			} else {
				subTask.Display.Template = lineParallelTemplate
			}
			finalTasks = append(finalTasks, *subTask)
		}
	}

	// replace parallel tasks with the inflated list of final tasks
	task.ParallelTasks = finalTasks
}

func (task *Task) inflate(displayIdx int, replicaValue string) {
	cmdString := task.CmdString
	name := task.Name

	if cmdString == "" && len(task.ParallelTasks) == 0 {
		exitWithErrorMessage("Task '" + name + "' misconfigured (A configured task must have at least either 'cmd' or 'parallel-tasks' configured)")
	}

	if replicaValue != "" {
		cmdString = strings.Replace(cmdString, config.Options.ReplicaReplaceString, replicaValue, -1)
	}

	task.CmdString = cmdString

	if eta, ok := config.commandTimeCache[task.CmdString]; ok {
		task.Command.EstimatedRuntime = eta
	} else {
		task.Command.EstimatedRuntime = time.Duration(-1)
	}

	command := strings.Split(cmdString, " ")
	task.Command.Cmd = cmd.NewCmd(command[0], command[1:]...)
	task.Command.ReturnCode = -1
	task.Display.Template = lineDefaultTemplate
	task.Display.Index = displayIdx
	task.ErrorBuffer = bytes.NewBufferString("")

	// set the name
	if name == "" {
		task.Name = cmdString
	} else {
		if replicaValue != "" {
			name = strings.Replace(name, config.Options.ReplicaReplaceString, replicaValue, -1)
		}
		task.Name = name
	}

}

func (task *Task) Tasks() (tasks []*Task) {
	if task.CmdString != "" {
		tasks = append(tasks, task)
	} else {
		for nestIdx := range task.ParallelTasks {
			tasks = append(tasks, &task.ParallelTasks[nestIdx])
		}
	}
	return tasks
}

func (task *Task) String() string {

	if task.Command.Complete {
		task.Display.Values.Spinner = ""
		task.Display.Values.Eta = ""
		if task.Command.ReturnCode != 0 && task.IgnoreFailure == false {
			task.Display.Values.Msg = red("Exited with error (" + strconv.Itoa(task.Command.ReturnCode) + ")")
		}
	}

	// set the name
	if task.Name == "" {
		if len(task.CmdString) > 25 {
			task.Name = task.CmdString[:22] + "..."
		} else {
			task.Name = task.CmdString
		}
	}

	// display
	var message bytes.Buffer
	terminalWidth, _ := terminal.Width()

	// get a string with the summary line without a split gap or message
	task.Display.Values.Split = ""
	originalMessage := task.Display.Values.Msg
	task.Display.Values.Msg = ""
	task.Display.Template.Execute(&message, task.Display.Values)

	// calculate the max width of the message and trim it
	maxMessageWidth := int(terminalWidth) - visualLength(message.String())
	task.Display.Values.Msg = originalMessage
	if visualLength(task.Display.Values.Msg) > maxMessageWidth {
		task.Display.Values.Msg = trimToVisualLength(task.Display.Values.Msg, maxMessageWidth-3) + "..."
	}

	// calculate a space buffer to push the eta to the right
	message.Reset()
	task.Display.Template.Execute(&message, task.Display.Values)
	splitWidth := int(terminalWidth) - visualLength(message.String())
	if splitWidth < 0 {
		splitWidth = 0
	}

	message.Reset()
	task.Display.Values.Split = strings.Repeat(" ", splitWidth)
	task.Display.Template.Execute(&message, task.Display.Values)

	return message.String()
}

func (task *Task) display() {
	Screen().Display(task.String(), task.Display.Index)
}

func (task *Task) EstimatedRuntime() float64 {
	var etaSeconds float64
	// finalize task by appending to the set of final tasks
	if task.CmdString != "" && task.Command.EstimatedRuntime != -1 {
		etaSeconds += task.Command.EstimatedRuntime.Seconds()
	}

	var maxParallelEstimatedRuntime float64
	var taskEndSecond []float64
	var currentSecond float64
	var remainingParallelTasks = config.Options.MaxParallelCmds

	for subIndex := range task.ParallelTasks {
		subTask := &task.ParallelTasks[subIndex]
		if subTask.CmdString != "" && subTask.Command.EstimatedRuntime != -1 {
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
	waiter.Add(1)
	resultChan <- CmdIR{Task: task, Status: StatusRunning, ReturnCode: -1}
	ticker := time.NewTicker(250 * time.Millisecond)

	logToMain("Started Task: "+task.Name, INFO_FORMAT)

	tempFile, _ := ioutil.TempFile(config.logCachePath, "")
	task.LogFile = tempFile
	task.LogChan = make(chan LogItem)
	go SingleLogger(task.LogChan, task.Name, tempFile.Name())

	task.Command.StartTime = time.Now()
	statusChan := task.Command.Cmd.Start()
	running := true

	for running {
		select {

		case <-ticker.C:
			status := task.Command.Cmd.Status()

			resultChan <- CmdIR{Task: task, Status: StatusRunning, Stdout: strconv.Itoa(len(status.Stdout)) + " : " + status.Stdout[len(status.Stdout)-1], ReturnCode: -1}

		case finalStatus := <-statusChan:
			// the subprocess has completeds
			ticker.Stop()
			running = false
			finalStatusStr := StatusSuccess
			if finalStatus.Exit != 0 {
				finalStatusStr = StatusError
				if task.StopOnFailure {
					exitSignaled = true
				}
			}
			resultChan <- CmdIR{Task: task, Status: finalStatusStr, ReturnCode: task.Command.Cmd.Status().Exit, Complete: true}
		}

	}
	task.Command.StopTime = time.Now()
	//logToMain("Completed Task: "+task.Name+" (rc: "+returnCodeMsg+")", INFO_FORMAT)
	waiter.Done()
}

// func (task *Task) runSingleCmd(resultChan chan CmdIR, waiter *sync.WaitGroup) {
// logToMain("Started Task: "+task.Name, INFO_FORMAT)

// task.Command.StartTime = time.Now()

// resultChan <- CmdIR{Task: task, Status: StatusRunning, ReturnCode: -1}
// waiter.Add(1)
// defer waiter.Done()

// tempFile, _ := ioutil.TempFile(config.logCachePath, "")
// task.LogFile = tempFile
// task.LogChan = make(chan LogItem)
// go SingleLogger(task.LogChan, task.Name, tempFile.Name())

// 	task.Command.Cmd.Start()

// 	readPipe := func(resultChan chan string, pipe io.ReadCloser) {
// 		defer close(resultChan)

// 		scanner := bufio.NewScanner(pipe)
// 		scanner.Split(variableSplitFunc)
// 		for scanner.Scan() {
// 			message := scanner.Text()
// 			resultChan <- vtclean.Clean(message, false)
// 		}
// 	}

// 	stderrChan := make(chan string)
// 	go readPipe(stderrChan, stderrPipe)

// 	for {
// 		select {
// 		case stdoutMsg, ok := <-stdoutChan:
// 			if ok {
// 				resultChan <- CmdIR{Task: task, Status: StatusRunning, Stdout: stdoutMsg, ReturnCode: -1}
// 			} else {
// 				stdoutChan = nil
// 			}
// 		case stderrMsg, ok := <-stderrChan:
// 			if ok {
// 				resultChan <- CmdIR{Task: task, Status: StatusRunning, Stderr: stderrMsg, ReturnCode: -1}
// 			} else {
// 				stderrChan = nil
// 			}
// 		}
// 		if stdoutChan == nil && stderrChan == nil {
// 			break
// 		}
// 	}

// 	returnCode := 0
// 	returnCodeMsg := "unknown"
// 	if err := task.Command.Cmd.Wait(); err != nil {
// 		if exiterr, ok := err.(*exec.ExitError); ok {
// 			// The program has exited with an exit code != 0
// 			if status, ok := exiterr.Sys().(syscall.WaitStatus); ok {
// 				returnCode = status.ExitStatus()
// 			}
// 		} else {
// 			returnCode = -1
// 			returnCodeMsg = "Failed to run: " + err.Error()
// 			resultChan <- CmdIR{Task: task, Status: StatusError, Stderr: returnCodeMsg, ReturnCode: returnCode}
// 		}
// 	}
// 	task.Command.StopTime = time.Now()

// 	logToMain("Completed Task: "+task.Name+" (rc: "+returnCodeMsg+")", INFO_FORMAT)

// 	if returnCode == 0 || task.IgnoreFailure {
// 		resultChan <- CmdIR{Task: task, Status: StatusSuccess, Complete: true, ReturnCode: returnCode}
// 	} else {
// 		resultChan <- CmdIR{Task: task, Status: StatusError, Complete: true, ReturnCode: returnCode}
// 		if task.StopOnFailure {
// 			exitSignaled = true
// 		}
// 	}
// }

func (task *Task) RunAndDisplay() []*Task {

	var (
		lastStartedTask int
		failedTasks     []*Task
		waiter          sync.WaitGroup
		message         bytes.Buffer
	)

	resultChan := make(chan CmdIR)
	tasks := task.Tasks()
	scr := Screen()
	hasHeader := len(tasks) > 1
	scr.ResetFrame(len(tasks), hasHeader, config.Options.ShowSummaryFooter)

	if !config.Options.Vintage {

		// make room for the title of a parallel proc group
		if hasHeader {

			message.Reset()
			lineObj := LineInfo{Status: StatusRunning.Color("i"), Title: task.Name, Msg: ""}
			task.Display.Template.Execute(&message, lineObj)
			scr.DisplayHeader(message.String())
		}

		for line := 0; line < len(tasks); line++ {
			tasks[line].Command.Started = false
			tasks[line].Display.Values = LineInfo{Status: StatusPending.Color("i"), Title: tasks[line].Name}
			tasks[line].display()
		}
	}

	var runningCmds int
	for ; lastStartedTask < config.Options.MaxParallelCmds && lastStartedTask < len(tasks); lastStartedTask++ {
		if config.Options.Vintage {
			fmt.Println(bold(task.Name + " : " + tasks[lastStartedTask].Name))
			fmt.Println(bold("Command: " + tasks[lastStartedTask].CmdString))
		}
		go tasks[lastStartedTask].runSingleCmd(resultChan, &waiter)
		tasks[lastStartedTask].Command.Started = true
		runningCmds++
	}
	groupSuccess := StatusSuccess

	// just wait for stuff to come back
	for runningCmds > 0 {
		select {
		case <-ticker.C:
			spinner.Next()

			for _, taskObj := range tasks {
				if taskObj.Command.Complete || !taskObj.Command.Started {
					taskObj.Display.Values.Spinner = ""
				} else {
					taskObj.Display.Values.Spinner = spinner.Current()
				}
				taskObj.display()
			}

			// update the summary line
			if config.Options.ShowSummaryFooter {
				scr.DisplayFooter(footer(StatusPending))
			}

		case msgObj := <-resultChan:
			eventTask := msgObj.Task

			// update the state before displaying...
			if msgObj.Complete {
				completedTasks++
				eventTask.Command.Complete = true
				eventTask.Command.ReturnCode = msgObj.ReturnCode
				close(eventTask.LogChan)

				config.commandTimeCache[eventTask.CmdString] = eventTask.Command.StopTime.Sub(eventTask.Command.StartTime)

				runningCmds--
				// if a thread has freed up, start the next task (if there are any left)
				if lastStartedTask < len(tasks) {
					if config.Options.Vintage {
						fmt.Println(bold(task.Name + " : " + tasks[lastStartedTask].Name))
						fmt.Println("Command: " + bold(tasks[lastStartedTask].CmdString))
					}
					go tasks[lastStartedTask].runSingleCmd(resultChan, &waiter)
					tasks[lastStartedTask].Command.Started = true
					runningCmds++
					lastStartedTask++
				}

				if msgObj.Status == StatusError {
					// update the group status to indicate a failed subtask
					groupSuccess = StatusError

					// keep note of the failed task for an after task report
					failedTasks = append(failedTasks, eventTask)
				}
			}

			// record in the log
			if config.Options.LogPath != "" {
				if msgObj.Stdout != "" {
					eventTask.LogChan <- LogItem{Name: eventTask.Name, Message: msgObj.Stdout + "\n"}
				}
				if msgObj.Stderr != "" {
					eventTask.LogChan <- LogItem{Name: eventTask.Name, Message: red(msgObj.Stderr) + "\n"}
				}
			}

			// keep record of all stderr lines for an after task report
			if msgObj.Stderr != "" {
				eventTask.ErrorBuffer.WriteString(msgObj.Stderr + "\n")
			}

			// display...
			if config.Options.Vintage {
				if msgObj.Stderr != "" {
					fmt.Println(red(msgObj.Stderr))
				} else {
					fmt.Println(msgObj.Stdout)
				}
			} else {
				if eventTask.ShowTaskOutput == false {
					msgObj.Stderr = ""
					msgObj.Stdout = ""
				}

				if msgObj.Stderr != "" {
					eventTask.Display.Values = LineInfo{Status: msgObj.Status.Color("i"), Title: eventTask.Name, Msg: red(msgObj.Stderr), Spinner: spinner.Current(), Eta: eventTask.CurrentEta()}
				} else {
					eventTask.Display.Values = LineInfo{Status: msgObj.Status.Color("i"), Title: eventTask.Name, Msg: blue(msgObj.Stdout), Spinner: spinner.Current(), Eta: eventTask.CurrentEta()}
				}

				eventTask.display()
			}

			// update the summary line
			if config.Options.ShowSummaryFooter {
				scr.DisplayFooter(footer(StatusPending))
			} else {
				scr.MovePastFrame(false)
			}

			if exitSignaled {
				break
			}

		}

	}

	if !exitSignaled {
		waiter.Wait()
	}

	if !config.Options.Vintage {
		// complete the proc group status
		if hasHeader {
			message.Reset()
			task.Display.Template.Execute(&message, LineInfo{Status: groupSuccess.Color("i"), Title: task.Name + purple(" ("+strconv.Itoa(len(tasks))+" tasks)")})
			scr.DisplayHeader(message.String())
		}

		// collapse sections or parallel tasks...
		if config.Options.CollapseOnCompletion && len(tasks) > 1 {
			// erase the lines for this section (except for the header)

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

	return failedTasks
}
