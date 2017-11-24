package main

import (
	"bytes"
	"fmt"
	"html/template"
	"math/rand"
	"os"
	"strconv"
	"time"

	ansi "github.com/k0kubun/go-ansi"
	//"github.com/k0kubun/pp"
	color "github.com/mgutz/ansi"
	terminal "github.com/wayneashleyberry/terminal-dimensions"
)

var (
	Options       OptionsConfig
	ExitSignaled  = false
	purple        = color.ColorFunc("magenta+h")
	red           = color.ColorFunc("red+h")
	green         = color.ColorFunc("green")
	bold          = color.ColorFunc("default+b")
	normal        = color.ColorFunc("default")
	StatusSuccess = color.Color("  ", "green+ih")
	StatusError   = color.Color("  ", "red+ih")
	//StatusRunning               = color.Color("  ", "28+i")
	StatusRunning               = color.Color("  ", "22+i")
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

func footer(status string) string {
	var tpl bytes.Buffer
	percent := (float64(CompletedTasks) * float64(100)) / float64(TotalTasks)
	SummaryTemplate.Execute(&tpl, Summary{status, percent, ""})
	return tpl.String()
}

func main() {

	var conf RunConfig
	conf.read()
	Options = conf.Options

	rand.Seed(time.Now().UnixNano())

	if Options.LogPath != "" {
		fmt.Println("Logging is not supported yet!")
		os.Exit(1)
		go logFlusher()
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
