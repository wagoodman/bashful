package main

import (
	"bytes"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	ansi "github.com/k0kubun/go-ansi"
	//"github.com/k0kubun/pp"
	"github.com/go-cmd/cmd"
	color "github.com/mgutz/ansi"
	spin "github.com/tj/go-spin"
	terminal "github.com/wayneashleyberry/terminal-dimensions"
	yaml "gopkg.in/yaml.v2"
)

var (
	Options       ConfigOptions
	ExitSignaled  = false
	purple        = color.ColorFunc("magenta+h")
	red           = color.ColorFunc("red+h")
	green         = color.ColorFunc("green")
	bold          = color.ColorFunc("default+b")
	normal        = color.ColorFunc("default")
	StatusSuccess = color.Color("  ", "green+ih")
	StatusError   = color.Color("  ", "red+ih")
	StatusRunning = color.Color("  ", "28+i")
	//StatusRunning               = color.Color("  ", "22+i")
	StatusPending               = color.Color("  ", "22+i")
	SummaryPendingArrow         = color.Color("    ", "22+i")     //color.Color("    ", "22+i")     //+ color.Color("❯❯❯", "22")
	SummarySuccessArrow         = color.Color("    ", "green+ih") //color.Color("    ", "green+ih") //+ color.Color("❯❯❯", "green+h")
	SummaryFailedArrow          = color.Color("    ", "red+ih")
	LineDefaultTemplate, _      = template.New("default line").Parse(" {{.Status}} {{printf \"%1s\" .Spinner}} {{printf \"%-25s\" .Title}}       {{.Msg}}")
	LineParallelTemplate, _     = template.New("parallel line").Parse(" {{.Status}} {{printf \"%1s\" .Spinner}}  ├─ {{printf \"%-25s\" .Title}}   {{.Msg}}")
	LineLastParallelTemplate, _ = template.New("last parallel line").Parse(" {{.Status}} {{printf \"%1s\" .Spinner}}  └─ {{printf \"%-25s\" .Title}}   {{.Msg}}")
	LineErrorTemplate, _        = template.New("error line").Parse(" {{.Status}} {{.Msg}}")
	SummaryTemplate, _          = template.New("summary line").Parse(` {{.Status}}` + bold(` {{printf "%3.2f" .Percent}}% Complete {{.Msg}}`))
	TotalTasks                  = 0
	CompletedTasks              = 0
	LogChan                     = make(chan LogItem, 10000)
)

type ConfigOptions struct {
	StopOnFailure        bool   `yaml:"stop-on-failure"`
	ShowSteps            bool   `yaml:"show-steps"`
	ShowSummaryFooter    bool   `yaml:"show-summary-footer"`
	ShowFailureReport    bool   `yaml:"show-failure-summary"`
	LogPath              string `yaml:"log-path"`
	Vintage              bool   `yaml:"vintage"`
	MaxParallelCmds      int    `yaml:"max-parallel-commands"`
	ReplicaReplaceString string `yaml:"replica-replace-pattern"`
}

type TaskDisplay struct {
	Template *template.Template
	Idx      int
	Line     Line
}

type TaskCommand struct {
	Cmd        *cmd.Cmd
	Started    bool
	Complete   bool
	ReturnCode int
}

type Line struct {
	Status  string
	Title   string
	Msg     string
	Spinner string
}

type Summary struct {
	Status  string
	Percent float64
	Msg     string
}

type Task struct {
	Name          string `yaml:"name"`
	CmdString     string `yaml:"cmd"`
	Display       TaskDisplay
	Command       TaskCommand
	StopOnFailure bool     `yaml:"stop-on-failure"`
	ParallelTasks []Task   `yaml:"parallel-tasks"`
	ForEach       []string `yaml:"for-each"`
	waiter        sync.WaitGroup
	LogBuffer     *bytes.Buffer
	ErrorBuffer   *bytes.Buffer
}

type LogItem struct {
	Name    string
	Message string
}

type Config struct {
	Options ConfigOptions `yaml:"config"`
	Tasks   []Task        `yaml:"tasks"`
}

type CmdIR struct {
	Task      *Task
	Status    cmd.Status
	StatusStr string
}

type PipeIR struct {
	message string
}

// set default values for undefined yaml

func (obj *ConfigOptions) UnmarshalYAML(unmarshal func(interface{}) error) error {
	type defaults ConfigOptions
	var defaultValues defaults
	defaultValues.StopOnFailure = true
	defaultValues.ShowSteps = false
	defaultValues.ShowSummaryFooter = true
	defaultValues.ReplicaReplaceString = "?"
	defaultValues.MaxParallelCmds = 4

	if err := unmarshal(&defaultValues); err != nil {
		return err
	}

	*obj = ConfigOptions(defaultValues)
	// set global options
	Options = *obj
	return nil
}

func (obj *Task) UnmarshalYAML(unmarshal func(interface{}) error) error {
	type defaults Task
	var defaultValues defaults
	defaultValues.StopOnFailure = Options.StopOnFailure

	if err := unmarshal(&defaultValues); err != nil {
		return err
	}

	*obj = Task(defaultValues)
	return nil
}

func (task *Task) inflate(displayIdx int, replicaValue string) {
	cmdString := task.CmdString
	name := task.Name
	if replicaValue != "" {
		cmdString = strings.Replace(cmdString, Options.ReplicaReplaceString, replicaValue, -1)
		name = strings.Replace(name, Options.ReplicaReplaceString, replicaValue, -1)
	}
	command := strings.Split(cmdString, " ")
	// task.Command.Cmd = exec.Command(command[0], command[1:]...)
	task.Command.Cmd = cmd.NewCmd(command[0], command[1:]...)
	task.Command.ReturnCode = -1
	task.Display.Template = LineDefaultTemplate
	task.Display.Idx = displayIdx
	task.LogBuffer = bytes.NewBufferString("")
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

func (task *Task) create(displayStartIdx int, replicaValue string) {
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
				subTask.create(subReplicaIndex, subReplicaValue)

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

func (conf *Config) readConfig() {
	fmt.Println("Reading " + os.Args[1] + " ...")
	yamlString, err := ioutil.ReadFile(os.Args[1])
	if err != nil {
		log.Printf("yamlFile.Get err   #%v ", err)
	}

	err = yaml.Unmarshal(yamlString, conf)
	if err != nil {
		log.Fatalf("Unmarshal: %v", err)
	}

	var finalTasks []Task

	// initialize tasks with default values
	for index := range conf.Tasks {
		task := &conf.Tasks[index]
		// finalize task by appending to the set of final tasks
		if len(task.ForEach) > 0 {
			taskName, taskCmdString := task.Name, task.CmdString
			for _, replicaValue := range task.ForEach {
				task.Name = taskName
				task.CmdString = taskCmdString
				task.create(0, replicaValue)
				finalTasks = append(finalTasks, *task)
			}
		} else {
			task.create(0, "")
			finalTasks = append(finalTasks, *task)
		}
	}

	// replace the current config with the inflated list of final tasks
	conf.Tasks = finalTasks
}

func (task *Task) getParallelTasks() (tasks []*Task) {
	if task.CmdString != "" {
		tasks = append(tasks, task)
	} else {
		for nestIdx := range task.ParallelTasks {
			tasks = append(tasks, &task.ParallelTasks[nestIdx])
		}
	}
	return tasks
}

func visualLength(str string) int {
	inEscapeSeq := false
	length := 0

	for _, r := range str {
		switch {
		case inEscapeSeq:
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
				inEscapeSeq = false
			}
		case r == '\x1b':
			inEscapeSeq = true
		default:
			length++
		}
	}

	return length
}

func display(message string, curLine *int, targetIdx int) {
	moves := *curLine - targetIdx
	if moves != 0 {
		if moves < 0 {
			ansi.CursorDown(moves * -1)
		} else {
			ansi.CursorUp(moves)
		}
		*curLine -= moves
	}

	// trim message length
	terminalWidth, _ := terminal.Width()
	didShorten := false
	for visualLength(message) > int(terminalWidth-3) {
		message = message[:len(message)-3]
		didShorten = true
	}
	if didShorten {
		message += "..."
	}

	// display
	ansi.EraseInLine(2)
	// note: ansi cursor down cannot be used as this may be the last row
	fmt.Println(message)
	*curLine++
}

func (task *Task) display(curLine *int) {
	if task.Command.Complete {
		task.Display.Line.Spinner = ""
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

	// todo: by some ansi escape sequences

	// If at end of file with data return the data
	if atEOF {
		return len(data), data, nil
	}

	return
}

func (task *Task) runCmd(resultChan chan CmdIR, waiter *sync.WaitGroup) {
	waiter.Add(1)
	resultChan <- CmdIR{task, cmd.Status{}, StatusRunning}
	ticker := time.NewTicker(250 * time.Millisecond)

	statusChan := task.Command.Cmd.Start()
	running := true

	for running {
		select {

		case <-ticker.C:
			resultChan <- CmdIR{task, task.Command.Cmd.Status(), StatusRunning}

		case finalStatus := <-statusChan:
			// the subprocess has completeds
			ticker.Stop()
			running = false
			finalStatusStr := StatusSuccess
			if finalStatus.Exit != 0 {
				finalStatusStr = StatusError
				if task.StopOnFailure {
					ExitSignaled = true
				}
			}
			resultChan <- CmdIR{task, finalStatus, finalStatusStr}
		}

	}

	waiter.Done()
}

func footer(status string) string {
	var tpl bytes.Buffer
	percent := (float64(CompletedTasks) * float64(100)) / float64(TotalTasks)
	SummaryTemplate.Execute(&tpl, Summary{status, percent, ""})
	return tpl.String()
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
	tasks := task.getParallelTasks()

	if !Options.Vintage {
		if Options.ShowSteps {
			task.Name += color.ColorCode("reset") + " " + purple("〔"+strconv.Itoa(step)+"/"+strconv.Itoa(totalTasks)+"〕")
		}

		// make room for the title of a parallel proc group
		if len(tasks) > 1 {
			lineObj := Line{StatusRunning, bold(task.Name), "\n", ""}
			task.Display.Template.Execute(os.Stdout, lineObj)
		}

		for line := 0; line < len(tasks); line++ {
			tasks[line].Command.Started = false
			tasks[line].Display.Line = Line{StatusPending, tasks[line].Name, "", ""}
			tasks[line].display(&curLine)
		}
	}

	var runningCmds int
	for ; lastStartedTask < Options.MaxParallelCmds && lastStartedTask < len(tasks); lastStartedTask++ {
		if Options.Vintage {
			fmt.Println(bold(task.Name + " : " + tasks[lastStartedTask].Name))
			fmt.Println(bold("Command: " + tasks[lastStartedTask].CmdString))
		}
		go tasks[lastStartedTask].runCmd(resultChan, &task.waiter)
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
			if msgObj.Status.Complete {
				CompletedTasks++
				eventTask.Command.Complete = true
				eventTask.Command.ReturnCode = msgObj.Status.Exit
				if Options.LogPath != "" {
					LogChan <- LogItem{eventTask.Name, eventTask.LogBuffer.String()}
				}

				runningCmds--
				// if a thread has freed up, start the next task (if there are any left)
				if lastStartedTask < len(tasks) {
					if Options.Vintage {
						fmt.Println(bold(task.Name + " : " + tasks[lastStartedTask].Name))
						fmt.Println("Command: " + bold(tasks[lastStartedTask].CmdString))
					}
					go tasks[lastStartedTask].runCmd(resultChan, &task.waiter)
					tasks[lastStartedTask].Command.Started = true
					runningCmds++
					lastStartedTask++
				}

				if msgObj.Status.Exit != 0 {
					// update the group status to indicate a failed subtask
					groupSuccess = StatusError

					// keep note of the failed task for an after task report
					failedTasks = append(failedTasks, eventTask)
				}
			}

			// record in the log
			if Options.LogPath != "" {
				if len(msgObj.Status.Stdout) > 0 {
					for _, line := range msgObj.Status.Stdout {
						eventTask.LogBuffer.WriteString(line + "\n")
					}
				}
				if len(msgObj.Status.Stderr) > 0 {
					for _, line := range msgObj.Status.Stderr {
						eventTask.LogBuffer.WriteString(red(line) + "\n")
					}
				}
			}

			// keep record of all stderr lines for an after task report
			if len(msgObj.Status.Stderr) > 0 {
				for _, line := range msgObj.Status.Stderr {
					eventTask.ErrorBuffer.WriteString(line + "\n")
				}
			}

			// display...
			if Options.Vintage {
				if len(msgObj.Status.Stdout) > 0 {
					for _, line := range msgObj.Status.Stdout {
						fmt.Println(line)
					}
				} else if len(msgObj.Status.Stderr) > 0 {
					for _, line := range msgObj.Status.Stderr {
						fmt.Println(red(line))
					}
				}
			} else {
				if len(msgObj.Status.Stdout) > 0 {
					eventTask.Display.Line = Line{msgObj.StatusStr,
						eventTask.Name,
						msgObj.Status.Stdout[len(msgObj.Status.Stdout)-1],
						spinner.Current()}

					// if the command has completed, so no output in the normal display mode
					if msgObj.Status.Complete {
						eventTask.Display.Line.Msg = ""
					}

					eventTask.display(&curLine)
				} else if len(msgObj.Status.Stderr) > 0 {
					eventTask.Display.Line = Line{msgObj.StatusStr,
						eventTask.Name,
						red(msgObj.Status.Stderr[len(msgObj.Status.Stderr)-1]),
						spinner.Current()}

					// if the command has completed, so no output in the normal display mode
					if msgObj.Status.Complete {
						eventTask.Display.Line.Msg = ""
					}

					eventTask.display(&curLine)
				}
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
		task.waiter.Wait()
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
			task.Display.Template.Execute(os.Stdout, Line{groupSuccess, bold(task.Name), "", ""})
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

func logFlusher() {
	//create your file with desired read/write permissions
	f, err := os.OpenFile(Options.LogPath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		log.Fatal(err)
	}

	//defer to close when you're done with it, not because you think it's idiomatic!
	defer f.Close()

	//set output of logs to f
	log.SetOutput(f)

	//test case
	log.Println(bold("Started!"))

	for {
		select {
		case logObj := <-LogChan:
			log.Println(bold("Output from :"+logObj.Name) + "\n" + logObj.Message)
		}
	}
}

func main() {

	var conf Config
	conf.readConfig()

	rand.Seed(time.Now().UnixNano())

	if Options.LogPath != "" {
		fmt.Println("Logging is not supported yet!")
		os.Exit(1)
		// go logFlusher()
	}

	if Options.Vintage {
		Options.MaxParallelCmds = 1
		Options.ShowSummaryFooter = false
		Options.ShowFailureReport = false
	}

	var failedTasks []*Task

	fmt.Print("\033[?25l") // hide cursor
	for index := range conf.Tasks {
		failedTasks = append(failedTasks, conf.Tasks[index].process(index+1, len(conf.Tasks))...)

		if ExitSignaled {
			break
		}
	}
	var curLine int

	if Options.ShowSummaryFooter {
		if len(failedTasks) > 0 {
			display(footer(SummaryFailedArrow), &curLine, 0)
		} else {
			display(footer(SummarySuccessArrow), &curLine, 0)
		}
	}

	if Options.ShowFailureReport {
		fmt.Println("Some tasks failed, see below for details.\n")
		for _, task := range failedTasks {
			fmt.Println(bold(red("Failed task: " + task.Name)))
			fmt.Println(" ├─ command: " + task.CmdString)
			fmt.Println(" ├─ return code: " + strconv.Itoa(task.Command.ReturnCode))
			fmt.Println(" └─ stderr: \n" + task.ErrorBuffer.String())
			fmt.Println()
		}
	}

	fmt.Print("\033[?25h") // show cursor

}
