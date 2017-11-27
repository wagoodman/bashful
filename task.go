package main

import (
	"bufio"
	"bytes"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	ansi "github.com/k0kubun/go-ansi"
	"github.com/lunixbochs/vtclean"
	color "github.com/mgutz/ansi"
	spin "github.com/tj/go-spin"
	terminal "github.com/wayneashleyberry/terminal-dimensions"
)

type Task struct {
	Name          string `yaml:"name"`
	CmdString     string `yaml:"cmd"`
	Display       TaskDisplay
	Command       TaskCommand
	StopOnFailure bool     `yaml:"stop-on-failure"`
	ParallelTasks []Task   `yaml:"parallel-tasks"`
	ForEach       []string `yaml:"for-each"`
	LogChan       chan LogItem
	LogFile       *os.File
	ErrorBuffer   *bytes.Buffer
}

type TaskDisplay struct {
	Template *template.Template
	Idx      int
	Line     Line
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

type CmdIR struct {
	Task       *Task
	Status     string
	Stdout     string
	Stderr     string
	Complete   bool
	ReturnCode int
}

type PipeIR struct {
	message string
}

type Line struct {
	Status  string
	Title   string
	Msg     string
	Spinner string
	Eta     string
}

func (task *Task) UnmarshalYAML(unmarshal func(interface{}) error) error {
	type defaults Task
	var defaultValues defaults
	defaultValues.StopOnFailure = Options.StopOnFailure

	if err := unmarshal(&defaultValues); err != nil {
		return err
	}

	*task = Task(defaultValues)
	return nil
}

func (task *Task) Create(displayStartIdx int, replicaValue string) {
	task.inflate(displayStartIdx, replicaValue)

	if task.CmdString != "" {
		TotalTasks++
	}

	var finalTasks []Task

	for subIndex := range task.ParallelTasks {
		subTask := &task.ParallelTasks[subIndex]

		if len(subTask.ForEach) > 0 {
			subTaskName, subTaskCmdString := subTask.Name, subTask.CmdString
			for subReplicaIndex, subReplicaValue := range subTask.ForEach {
				//subTaskReplica := Task{}
				subTask.Name = subTaskName
				subTask.CmdString = subTaskCmdString
				subTask.Create(subReplicaIndex, subReplicaValue)

				if subReplicaIndex == len(subTask.ForEach)-1 {
					subTask.Display.Template = LineLastParallelTemplate
				} else {
					subTask.Display.Template = LineParallelTemplate
				}

				finalTasks = append(finalTasks, *subTask)
			}
		} else {
			subTask.inflate(subIndex, replicaValue)
			TotalTasks++

			if subIndex == len(task.ParallelTasks)-1 {
				subTask.Display.Template = LineLastParallelTemplate
			} else {
				subTask.Display.Template = LineParallelTemplate
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

	if replicaValue != "" {
		cmdString = strings.Replace(cmdString, Options.ReplicaReplaceString, replicaValue, -1)
		name = strings.Replace(name, Options.ReplicaReplaceString, replicaValue, -1)
	}

	task.CmdString = cmdString

	if eta, ok := CommandTimeCache[task.CmdString]; ok {
		task.Command.EstimatedRuntime = eta
	} else {
		task.Command.EstimatedRuntime = time.Duration(-1)
	}

	command := strings.Split(cmdString, " ")
	task.Command.Cmd = exec.Command(command[0], command[1:]...)
	task.Command.ReturnCode = -1
	task.Display.Template = LineDefaultTemplate
	task.Display.Idx = displayIdx
	task.ErrorBuffer = bytes.NewBufferString("")

	// set the name
	if name == "" {
		if len(cmdString) > 25 {
			task.Name = cmdString[:20] + "..."
		} else {
			task.Name = cmdString
		}
	} else {
		task.Name = name
	}
}

func (task *Task) tasks() (tasks []*Task) {
	if task.CmdString != "" {
		tasks = append(tasks, task)
	} else {
		for nestIdx := range task.ParallelTasks {
			tasks = append(tasks, &task.ParallelTasks[nestIdx])
		}
	}
	return tasks
}

func (task *Task) display(curLine *int) {
	if task.Command.Complete {
		task.Display.Line.Spinner = ""
		task.Display.Line.Eta = ""
		if task.Command.ReturnCode != 0 {
			task.Display.Line.Msg = red("Exited with error (" + strconv.Itoa(task.Command.ReturnCode) + ")")
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
	task.Display.Template.Execute(&message, task.Display.Line)
	display(message.String(), curLine, task.Display.Idx)
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

func (task *Task) run(resultChan chan CmdIR, waiter *sync.WaitGroup) {
	task.Command.StartTime = time.Now()
	MainLogChan <- LogItem{task.Name, "Running Cmd: " + task.CmdString}
	resultChan <- CmdIR{task, StatusRunning, "", "", false, -1}
	waiter.Add(1)
	defer waiter.Done()

	tempFile, _ := ioutil.TempFile(LogCachePath, "")
	task.LogFile = tempFile
	task.LogChan = make(chan LogItem)
	go SingleLogger(task.LogChan, task.Name, tempFile.Name())

	stdoutPipe, _ := task.Command.Cmd.StdoutPipe()
	stderrPipe, _ := task.Command.Cmd.StderrPipe()

	task.Command.Cmd.Start()

	readPipe := func(resultChan chan PipeIR, pipe io.ReadCloser) {
		defer close(resultChan)

		scanner := bufio.NewScanner(pipe)
		scanner.Split(variableSplitFunc)
		for scanner.Scan() {
			message := scanner.Text()
			resultChan <- PipeIR{vtclean.Clean(message, false)}
		}
	}

	stdoutChan := make(chan PipeIR)
	stderrChan := make(chan PipeIR)
	go readPipe(stdoutChan, stdoutPipe)
	go readPipe(stderrChan, stderrPipe)

	for {
		select {
		case stdoutMsg, ok := <-stdoutChan:
			if ok {
				resultChan <- CmdIR{task, StatusRunning, stdoutMsg.message, "", false, -1}
			} else {
				stdoutChan = nil
			}
		case stderrMsg, ok := <-stderrChan:
			if ok {
				resultChan <- CmdIR{task, StatusRunning, "", stderrMsg.message, false, -1}
			} else {
				stderrChan = nil
			}
		}
		if stdoutChan == nil && stderrChan == nil {
			break
		}
	}

	var waitStatus syscall.WaitStatus

	err := task.Command.Cmd.Wait()
	task.Command.StopTime = time.Now()

	if exitError, ok := err.(*exec.ExitError); ok {
		waitStatus = exitError.Sys().(syscall.WaitStatus)
	} else {
		waitStatus = task.Command.Cmd.ProcessState.Sys().(syscall.WaitStatus)
	}

	returnCode := waitStatus.ExitStatus()

	if returnCode == 0 {
		resultChan <- CmdIR{task, StatusSuccess, "", "", true, returnCode}
	} else {
		resultChan <- CmdIR{task, StatusError, "", "", true, returnCode}
		if task.StopOnFailure {
			ExitSignaled = true
		}
	}
}

func (task *Task) eta() string {
	var eta, etaValue string

	if Options.ShowTaskEta {
		running := time.Since(task.Command.StartTime)
		etaValue = "?"
		if task.Command.EstimatedRuntime > 0 {
			etaValue = showDuration(time.Duration(task.Command.EstimatedRuntime.Seconds()-running.Seconds()) * time.Second)
		}
		eta = fmt.Sprintf("Eta[%s] ", etaValue)
	}
	return eta
}

func (task *Task) process(step, totalTasks int) []*Task {

	var (
		curLine         int
		lastStartedTask int
		moves           int
		failedTasks     []*Task
	)

	spinner := spin.New()
	ticker := time.NewTicker(150 * time.Millisecond)
	if Options.Vintage {
		ticker.Stop()
	}
	resultChan := make(chan CmdIR, 10000)
	tasks := task.tasks()
	var waiter sync.WaitGroup

	if !Options.Vintage {
		if Options.ShowSteps {
			task.Name += color.ColorCode("reset") + " " + purple("〔"+strconv.Itoa(step)+"/"+strconv.Itoa(totalTasks)+"〕")
		}

		// make room for the title of a parallel proc group
		if len(tasks) > 1 {
			lineObj := Line{StatusRunning, bold(task.Name), "\n", "", ""}
			task.Display.Template.Execute(os.Stdout, lineObj)
		}

		for line := 0; line < len(tasks); line++ {
			tasks[line].Command.Started = false
			tasks[line].Display.Line = Line{StatusPending, tasks[line].Name, "", "", ""}
			tasks[line].display(&curLine)
		}
	}

	var runningCmds int
	for ; lastStartedTask < Options.MaxParallelCmds && lastStartedTask < len(tasks); lastStartedTask++ {
		if Options.Vintage {
			fmt.Println(bold(task.Name + " : " + tasks[lastStartedTask].Name))
			fmt.Println(bold("Command: " + tasks[lastStartedTask].CmdString))
		}
		go tasks[lastStartedTask].run(resultChan, &waiter)
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
					taskObj.Display.Line.Spinner = ""
				} else {
					taskObj.Display.Line.Spinner = spinner.Current()
				}
				taskObj.display(&curLine)
			}

			// update the summary line
			if Options.ShowSummaryFooter {
				display(footer(SummaryPendingArrow), &curLine, len(tasks))
			}

		case msgObj := <-resultChan:
			eventTask := msgObj.Task

			// update the state before displaying...
			if msgObj.Complete {
				CompletedTasks++
				eventTask.Command.Complete = true
				eventTask.Command.ReturnCode = msgObj.ReturnCode
				close(eventTask.LogChan)

				CommandTimeCache[eventTask.CmdString] = eventTask.Command.StopTime.Sub(eventTask.Command.StartTime)

				runningCmds--
				// if a thread has freed up, start the next task (if there are any left)
				if lastStartedTask < len(tasks) {
					if Options.Vintage {
						fmt.Println(bold(task.Name + " : " + tasks[lastStartedTask].Name))
						fmt.Println("Command: " + bold(tasks[lastStartedTask].CmdString))
					}
					go tasks[lastStartedTask].run(resultChan, &waiter)
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
			if Options.LogPath != "" {
				if msgObj.Stdout != "" {
					eventTask.LogChan <- LogItem{eventTask.Name, msgObj.Stdout + "\n"}
				}
				if msgObj.Stderr != "" {
					eventTask.LogChan <- LogItem{eventTask.Name, red(msgObj.Stderr) + "\n"}
				}
			}

			// keep record of all stderr lines for an after task report
			if msgObj.Stderr != "" {
				eventTask.ErrorBuffer.WriteString(msgObj.Stderr + "\n")
			}

			// display...
			if Options.Vintage {
				if msgObj.Stderr != "" {
					fmt.Println(red(msgObj.Stderr))
				} else {
					fmt.Println(msgObj.Stdout)
				}
			} else {
				if msgObj.Stderr != "" {
					eventTask.Display.Line = Line{msgObj.Status, eventTask.Name, red(msgObj.Stderr), spinner.Current(), eventTask.eta()}
				} else {
					eventTask.Display.Line = Line{msgObj.Status, eventTask.Name, msgObj.Stdout, spinner.Current(), eventTask.eta()}
				}

				eventTask.display(&curLine)
			}

			// update the summary line
			if Options.ShowSummaryFooter {
				display(footer(SummaryPendingArrow), &curLine, len(tasks))
			}

			if ExitSignaled {
				break
			}

		}

	}

	if !ExitSignaled {
		waiter.Wait()
	}

	if !Options.Vintage {
		// complete the proc group status
		if len(tasks) > 1 {

			moves = curLine + 1
			if moves != 0 {
				if moves < 0 {
					ansi.CursorDown(moves * -1)
				} else {
					ansi.CursorUp(moves)
				}
				curLine -= moves
			}

			ansi.EraseInLine(2)
			task.Display.Template.Execute(os.Stdout, Line{groupSuccess, bold(task.Name), "", "", ""})
			ansi.CursorHorizontalAbsolute(0)
		}

		// reset the cursor to the bottom of the section
		moves = curLine - len(tasks)
		if moves != 0 {
			if moves < 0 {
				ansi.CursorDown(moves * -1)
			} else {
				ansi.CursorUp(moves)
			}
			curLine -= moves
		}
	}

	return failedTasks
}
