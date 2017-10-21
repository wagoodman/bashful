package main

import (
	"bufio"
	"bytes"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	ansi "github.com/k0kubun/go-ansi"
	//"github.com/k0kubun/pp"
	"github.com/lunixbochs/vtclean"
	color "github.com/mgutz/ansi"
	spin "github.com/tj/go-spin"
	terminal "github.com/wayneashleyberry/terminal-dimensions"
	yaml "gopkg.in/yaml.v2"
)

type Line struct {
	Status  string
	Title   string
	Msg     string
	Spinner string
}

const (
	MaxParallelCmds = 4
)

var (
	red                         = color.ColorFunc("red+h")
	green                       = color.ColorFunc("green")
	bold                        = color.ColorFunc("default+b")
	StatusSuccess               = color.Color("  ", "green+ih")
	StatusError                 = color.Color("  ", "red+ih")
	StatusRunning               = color.Color("  ", "28+i")
	StatusPending               = color.Color("  ", "22+i")
	LineDefaultTemplate, _      = template.New("default line").Parse(" {{.Status}} {{printf \"%1s\" .Spinner}} {{printf \"%-25s\" .Title}}       {{.Msg}}")
	LineParallelTemplate, _     = template.New("parallel line").Parse(" {{.Status}} {{printf \"%1s\" .Spinner}}  ├─ {{printf \"%-25s\" .Title}}   {{.Msg}}")
	LineLastParallelTemplate, _ = template.New("last parallel line").Parse(" {{.Status}} {{printf \"%1s\" .Spinner}}  └─ {{printf \"%-25s\" .Title}}   {{.Msg}}")
	LineErrorTemplate, _        = template.New("error line").Parse(" {{.Status}} {{.Msg}}")
)

type ConfigOptions struct {
}

type ActionDisplay struct {
	Template *template.Template
	Idx      int
	Line     Line
}

type ActionCommand struct {
	Cmd        *exec.Cmd
	Complete   bool
	ReturnCode int
}

type Action struct {
	Name            string `yaml:"name"`
	CmdString       string `yaml:"cmd"`
	Display         ActionDisplay
	Command         ActionCommand
	StopOnFailure   bool     `yaml:"stop_on_failure"`
	ParallelActions []Action `yaml:"tasks"`
	waiter          sync.WaitGroup
}

type Config struct {
	Options ConfigOptions `yaml:"options"`
	Tasks   []Action      `yaml:"tasks"`
}

type CmdIR struct {
	Action     *Action
	Status     string
	Stdout     string
	Complete   bool
	ReturnCode int
}

type PipeIR struct {
	message string
}

// todo: make setAction function to clean and initialize fields instead of this odd loop....

func (conf *Config) getConfig() {
	fmt.Println("Reading " + os.Args[1] + " ...")
	yamlString, err := ioutil.ReadFile(os.Args[1])
	if err != nil {
		log.Printf("yamlFile.Get err   #%v ", err)
	}

	err = yaml.Unmarshal(yamlString, conf)
	if err != nil {
		log.Fatalf("Unmarshal: %v", err)
	}

	// initialize actions with default values
	for index := range conf.Tasks {
		action := &conf.Tasks[index]
		action.Display.Template = LineDefaultTemplate
		action.Display.Idx = 0

		// set the name
		if action.Name == "" {
			if len(action.CmdString) > 25 {
				action.Name = action.CmdString[:20] + "..."
			} else {
				action.Name = action.CmdString
			}
		}

		for subIndex := range action.ParallelActions {
			subAction := &action.ParallelActions[subIndex]
			subAction.Display.Template = LineDefaultTemplate
			subAction.Display.Idx = subIndex

			// set the name
			if subAction.Name == "" {
				if len(subAction.CmdString) > 25 {
					subAction.Name = subAction.CmdString[:20] + "..."
				} else {
					subAction.Name = subAction.CmdString
				}
			}

		}

	}

}

func readPipe(resultChan chan PipeIR, pipe io.ReadCloser) {
	scanner := bufio.NewScanner(pipe)

	for scanner.Scan() {
		message := scanner.Text()
		resultChan <- PipeIR{vtclean.Clean(message, false)}
	}
}

// actually use a cmd
func (action *Action) runCmd(resultChan chan CmdIR, waiter *sync.WaitGroup) {
	waiter.Add(1)
	resultChan <- CmdIR{action, StatusPending, "", false, -1}

	var waitStatus syscall.WaitStatus
	var returnCode int
	stdoutPipe, _ := action.Command.Cmd.StdoutPipe()
	stderrPipe, _ := action.Command.Cmd.StderrPipe()

	stdoutChan := make(chan PipeIR)
	stderrChan := make(chan PipeIR)

	go readPipe(stdoutChan, stdoutPipe)
	go readPipe(stderrChan, stderrPipe)

	action.Command.Cmd.Start()

	select {
	case stdoutMsg := <-stdoutChan:
		resultChan <- CmdIR{action, StatusRunning, stdoutMsg.message, false, -1}
	case stderrMsg := <-stderrChan:
		resultChan <- CmdIR{action, StatusRunning, stderrMsg.message, false, -1}
	}

	err := action.Command.Cmd.Wait()

	if exitError, ok := err.(*exec.ExitError); ok {
		waitStatus = exitError.Sys().(syscall.WaitStatus)
	} else {
		waitStatus = action.Command.Cmd.ProcessState.Sys().(syscall.WaitStatus)
	}
	returnCode = waitStatus.ExitStatus()

	// note, this is a possible race condition: this thread may be modifying action
	// while the main thread is reading these fields. Find a better way to do this.
	// (without locking... just because...)

	waiter.Done()

	if returnCode == 0 {
		resultChan <- CmdIR{action, StatusSuccess, "", true, returnCode}
	} else {
		resultChan <- CmdIR{action, StatusError, "", true, returnCode}
	}
}

func (action *Action) getParallelActions() (actions []*Action) {

	if action.CmdString != "" {
		command := strings.Split(action.CmdString, " ")
		action.Command.Cmd = exec.Command(command[0], command[1:]...)
		action.Command.ReturnCode = -1
		actions = append(actions, action)
	} else {
		for nestIdx := range action.ParallelActions {
			command := strings.Split(action.ParallelActions[nestIdx].CmdString, " ")
			action.ParallelActions[nestIdx].Command.Cmd = exec.Command(command[0], command[1:]...)
			action.ParallelActions[nestIdx].Command.ReturnCode = -1
			actions = append(actions, &action.ParallelActions[nestIdx])
			if nestIdx == len(action.ParallelActions)-1 {
				action.ParallelActions[nestIdx].Display.Template = LineLastParallelTemplate
			} else {
				action.ParallelActions[nestIdx].Display.Template = LineParallelTemplate
			}
		}
	}
	return actions
}

func (action *Action) display(curLine *int) {
	moves := *curLine - action.Display.Idx
	if moves != 0 {
		if moves < 0 {
			ansi.CursorDown(moves * -1)
		} else {
			ansi.CursorUp(moves)
		}
		*curLine -= moves
	}

	if action.Command.Complete {
		action.Display.Line.Spinner = ""
		if action.Command.ReturnCode != 0 {
			action.Display.Line.Msg = red("Exited with error (" + strconv.Itoa(action.Command.ReturnCode) + ")")
		}
	}

	// trim message length
	terminalWidth, _ := terminal.Width()
	dummyObj := action.Display.Line
	dummyObj.Msg = ""
	var tpl bytes.Buffer
	action.Display.Template.Execute(&tpl, dummyObj)
	maxLineLen := int(terminalWidth) - len(tpl.String())
	if len(action.Display.Line.Msg) > maxLineLen {
		action.Display.Line.Msg = action.Display.Line.Msg[:maxLineLen-5] + "..."
	}

	// set the name
	if action.Name == "" {
		if len(action.CmdString) > 25 {
			action.Name = action.CmdString[:20] + "..."
		} else {
			action.Name = action.CmdString
		}
	}

	// display
	ansi.EraseInLine(2)
	action.Display.Template.Execute(os.Stdout, action.Display.Line)
	ansi.CursorDown(1)
	ansi.CursorHorizontalAbsolute(0)
	*curLine++
}

// mimic a cmd and show valid output
func (action *Action) process() {

	var (
		curLine           int
		lastStartedAction int
		moves             int
	)

	spinner := spin.New()
	ticker := time.NewTicker(150 * time.Millisecond)
	resultChan := make(chan CmdIR)
	actions := action.getParallelActions()

	// make room for the title of a parallel proc group
	if len(actions) > 1 {
		lineObj := Line{StatusPending, bold(action.Name), "\n", ""}
		action.Display.Template.Execute(os.Stdout, lineObj)
	}

	for line := 0; line < len(actions); line++ {
		actions[line].Display.Line = Line{StatusPending, actions[line].Name, "initializing...", ""}
		actions[line].display(&curLine)
	}

	var runningCmds int
	for ; lastStartedAction < MaxParallelCmds && lastStartedAction < len(actions); lastStartedAction++ {
		go actions[lastStartedAction].runCmd(resultChan, &action.waiter)
		runningCmds++
	}
	groupSuccess := StatusSuccess
	// just wait for stuff to come back
	for runningCmds > 0 {
		select {
		case <-ticker.C:
			spinner.Next()

			for _, actionObj := range actions {
				if actionObj.Command.Complete {
					actionObj.Display.Line.Spinner = ""
				} else {
					actionObj.Display.Line.Spinner = spinner.Current()
				}
				actionObj.display(&curLine)
			}

		case msgObj := <-resultChan:
			eventAction := msgObj.Action

			// update the state before displaying...
			if msgObj.Complete {
				eventAction.Command.Complete = true
				eventAction.Command.ReturnCode = msgObj.ReturnCode

				runningCmds--
				// if a thread has freed up, start the next action (if there are any left)
				if lastStartedAction < len(actions) {
					go actions[lastStartedAction].runCmd(resultChan, &action.waiter)
					runningCmds++
					lastStartedAction++
				}
				// update the group status
				if msgObj.Status == StatusError {
					groupSuccess = StatusError
				}
			}

			// display...
			eventAction.Display.Line = Line{msgObj.Status, eventAction.Name, msgObj.Stdout, spinner.Current()}
			eventAction.display(&curLine)

		}

	}
	action.waiter.Wait()

	// complete the proc group status
	if len(actions) > 1 {

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
		action.Display.Template.Execute(os.Stdout, Line{groupSuccess, bold(action.Name), "", ""})
		ansi.CursorHorizontalAbsolute(0)
	}

	// reset the cursor to the bottom of the section
	moves = curLine - len(actions)
	if moves != 0 {
		if moves < 0 {
			ansi.CursorDown(moves * -1)
		} else {
			ansi.CursorUp(moves)
		}
		curLine -= moves
	}

}

func main() {

	var conf Config
	conf.getConfig()

	rand.Seed(time.Now().UnixNano())

	fmt.Print("\033[?25l") // hide cursor
	for index := range conf.Tasks {
		conf.Tasks[index].process()
	}
	fmt.Print("\033[?25h") // show cursor

}
