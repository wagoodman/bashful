package main

import (
	"bytes"
	"fmt"
	"html/template"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/k0kubun/go-ansi"
	//"github.com/k0kubun/pp"

	color "github.com/mgutz/ansi"
	terminal "github.com/wayneashleyberry/terminal-dimensions"
)

var (
	Options                     OptionsConfig
	logCachePath                string
	cachePath                   string
	etaCachePath                string
	exitSignaled                bool                     = false
	startTime                   time.Time                = time.Now()
	purple                      func(string) string      = color.ColorFunc("magenta+h")
	red                         func(string) string      = color.ColorFunc("red+h")
	yellow                      func(string) string      = color.ColorFunc("yellow+h")
	boldyellow                  func(string) string      = color.ColorFunc("yellow+b")
	boldcyan                    func(string) string      = color.ColorFunc("cyan+b")
	bold                        func(string) string      = color.ColorFunc("default+b")
	lineDefaultTemplate, _                               = template.New("default line").Parse(` {{.Status}}  ` + color.Reset + ` {{printf "%1s" .Spinner}} {{printf "%-25s" .Title}} {{.Msg}}{{.Split}}{{.Eta}}`)
	lineParallelTemplate, _                              = template.New("parallel line").Parse(` {{.Status}}  ` + color.Reset + ` {{printf "%1s" .Spinner}} ├─ {{printf "%-25s" .Title}} {{.Msg}}{{.Split}}{{.Eta}}`)
	lineLastParallelTemplate, _                          = template.New("last parallel line").Parse(` {{.Status}}  ` + color.Reset + ` {{printf "%1s" .Spinner}} └─ {{printf "%-25s" .Title}} {{.Msg}}{{.Split}}{{.Eta}}`)
	summaryTemplate, _                                   = template.New("summary line").Parse(` {{.Status}}    ` + color.Reset + ` {{printf "%-16s" .Percent}}` + color.Reset + ` {{.Steps}}{{.Errors}}{{.Msg}}{{.Split}}{{.Runtime}}{{.Eta}}`)
	totalTasks                  int                      = 0
	completedTasks              int                      = 0
	totalFailedTasks            int                      = 0
	mainLogChan                 chan LogItem             = make(chan LogItem)
	mainLogConcatChan           chan LogConcat           = make(chan LogConcat)
	commandTimeCache            map[string]time.Duration = make(map[string]time.Duration)
	totalEtaSeconds             float64
)

type Summary struct {
	Status           string
	Percent          string
	Msg              string
	Runtime          string
	Eta              string
	Split            string
	Steps            string
	Errors           string
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

func trimToVisualLength(message string, length int) string {
	for visualLength(message) > length {
		message = message[:len(message)-1]
	}
	return message
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
	for visualLength(message) > int(terminalWidth) {
		message = trimToVisualLength(message, int(terminalWidth)-3) + "..."
	}

	// display
	ansi.EraseInLine(2)
	// note: ansi cursor down cannot be used as this may be the last row
	fmt.Println(message)
	*curLine++
}

func showDuration(duration time.Duration) string {
	seconds := int64(duration.Seconds()) % 60
	minutes := int64(duration.Minutes()) % 60
	hours := int64(duration.Hours()) % 24
	return fmt.Sprintf("%02d:%02d:%02d", hours, minutes, seconds)
}

func footer(status CommandStatus) string {
	var tpl bytes.Buffer
	var durString, etaString, stepString, errorString string

	if Options.ShowSummaryTimes {
		duration := time.Since(startTime)
		durString = fmt.Sprintf(" Runtime[%s]", showDuration(duration))

		totalEta := time.Duration(totalEtaSeconds) * time.Second
		remainingEta := time.Duration(totalEta.Seconds()-duration.Seconds()) * time.Second
		etaString = fmt.Sprintf(" ETA[%s]", showDuration(remainingEta))
	}

	if completedTasks == totalTasks {
		etaString = ""
	}

	if Options.ShowStepSummary {
		stepString = fmt.Sprintf(" Tasks[%d/%d]", completedTasks, totalTasks)
	}

	if Options.ShowErrorSummary {
		errorString = fmt.Sprintf(" Errors[%d]", totalFailedTasks)
	}

	// get a string with the summary line without a split gap (eta floats left)
	percentValue := (float64(completedTasks) * float64(100)) / float64(totalTasks)
	percentStr := fmt.Sprintf("%3.2f%% Complete", percentValue)

	if completedTasks == totalTasks {
		percentStr = status.Color("b") + percentStr + color.Reset
	} else {
		percentStr = color.Color(percentStr, "default+b")
	}

	summaryTemplate.Execute(&tpl, Summary{Status: status.Color("i"), Percent: percentStr, Runtime: durString, Eta: etaString, Steps: stepString, Errors: errorString})

	// calculate a space buffer to push the eta to the right
	terminalWidth, _ := terminal.Width()
	splitWidth := int(terminalWidth) - visualLength(tpl.String())
	if splitWidth < 0 {
		splitWidth = 0
	}

	tpl.Reset()
	summaryTemplate.Execute(&tpl, Summary{Status: status.Color("i"), Percent: percentStr, Runtime: bold(durString), Eta: bold(etaString), Split: strings.Repeat(" ", splitWidth), Steps: bold(stepString), Errors: bold(errorString)})

	return tpl.String()
}

func doesFileExist(name string) bool {
	if _, err := os.Stat(name); err != nil {
		if os.IsNotExist(err) {
			return false
		}
	}
	return true
}

func main() {

	var conf RunConfig
	var err error
	conf.read()

	rand.Seed(time.Now().UnixNano())

	if Options.LogPath != "" {
		// fmt.Println("Logging is not supported yet!")
		// os.Exit(1)
		setupLogging()
	}

	if Options.Vintage {
		Options.MaxParallelCmds = 1
		Options.ShowSummaryFooter = false
		Options.ShowFailureReport = false
	}
	var failedTasks []*Task

	fmt.Print("\033[?25l") // hide cursor
	mainLogChan <- LogItem{Name: "[Main]", Message: boldcyan("Running " + os.Args[1])}
	for index := range conf.Tasks {
		newFailedTasks := conf.Tasks[index].Process()
		totalFailedTasks += len(newFailedTasks)

		failedTasks = append(failedTasks, newFailedTasks...)

		if exitSignaled {
			break
		}
	}
	mainLogChan <- LogItem{Name: "[Main]", Message: boldcyan("Finished " + os.Args[1])}

	err = Save(etaCachePath, &commandTimeCache)
	Check(err)

	var curLine int

	if Options.ShowSummaryFooter {
		if len(failedTasks) > 0 {
			display(footer(StatusError), &curLine, 0)
		} else {
			display(footer(StatusSuccess), &curLine, 0)
		}
	}

	if Options.ShowFailureReport && len(failedTasks) > 0 {
		var buffer bytes.Buffer
		buffer.WriteString(red(" ...Some tasks failed, see below for details.\n"))

		for _, task := range failedTasks {

			buffer.WriteString("\n")
			buffer.WriteString(bold(red("⏺ Failed task: ")) + bold(task.Name) + "\n")
			buffer.WriteString(red("  ├─ command: ") + task.CmdString + "\n")
			buffer.WriteString(red("  ├─ return code: ") + strconv.Itoa(task.Command.ReturnCode) + "\n")
			buffer.WriteString(red("  └─ stderr: \n") + task.ErrorBuffer.String() + "\n")

		}
		mainLogChan <- LogItem{Name: "[Main]", Message: buffer.String()}
		fmt.Print(buffer.String())

	}

	mainLogChan <- LogItem{Name: "[Main]", Message: boldcyan("Exiting")}

	fmt.Print("\033[?25h") // show cursor

}
