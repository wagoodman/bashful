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

	ansi "github.com/k0kubun/go-ansi"
	//"github.com/k0kubun/pp"

	color "github.com/mgutz/ansi"
	terminal "github.com/wayneashleyberry/terminal-dimensions"
)

var (
	Options            OptionsConfig
	LogCachePath       string
	CachePath          string
	EtaCachePath       string
	ExitSignaled       = false
	StartTime          = time.Now()
	purple             = color.ColorFunc("magenta+h")
	red                = color.ColorFunc("red+h")
	green              = color.ColorFunc("green")
	bold               = color.ColorFunc("default+b")
	normal             = color.ColorFunc("default")
	StatusSuccess      = color.Color("  ", "green+ih")
	StatusError        = color.Color("  ", "red+ih")
	FinalStatusPending = color.ColorCode("default+b")
	FinalStatusSuccess = color.ColorCode("green+bh")
	FinalStatusError   = color.ColorCode("red+bh")
	//StatusRunning               = color.Color("  ", "28+i")
	StatusRunning               = color.Color("  ", "22+i")
	StatusPending               = color.Color("  ", "22+i")
	SummaryPendingArrow         = color.Color("    ", "22+i")     //color.Color("    ", "22+i")     //+ color.Color("❯❯❯", "22")
	SummarySuccessArrow         = color.Color("    ", "green+ih") //color.Color("    ", "green+ih") //+ color.Color("❯❯❯", "green+h")
	SummaryFailedArrow          = color.Color("    ", "red+ih")
	LineDefaultTemplate, _      = template.New("default line").Parse(" {{.Status}} {{printf \"%1s\" .Spinner}} {{printf \"%-25s\" .Title}}     {{.Eta}}{{.Msg}}")
	LineParallelTemplate, _     = template.New("parallel line").Parse(" {{.Status}} {{printf \"%1s\" .Spinner}} ├─ {{printf \"%-25s\" .Title}} {{.Eta}}{{.Msg}}")
	LineLastParallelTemplate, _ = template.New("last parallel line").Parse(" {{.Status}} {{printf \"%1s\" .Spinner}} └─ {{printf \"%-25s\" .Title}} {{.Eta}}{{.Msg}}")
	LineErrorTemplate, _        = template.New("error line").Parse(" {{.Status}} {{.Msg}}")
	PercentTemplate, _          = template.New("summary percent").Parse(`{{printf "%3.2f" .Value}}% Complete`)
	SummaryTemplate, _          = template.New("summary line").Parse(` {{.Status}}` + color.Reset + ` {{.FinalStatusColor}}{{printf "%-24s" .Percent}}` + color.Reset + ` {{.Eta}}{{.Runtime}}{{.Msg}}`)
	TotalTasks                  = 0
	CompletedTasks              = 0
	MainLogChan                 = make(chan LogItem)
	MainLogConcatChan           = make(chan LogConcat)
	CommandTimeCache            = make(map[string]time.Duration)
	TotalEtaSeconds             float64
)

type Percent struct {
	Value float64
}

type Summary struct {
	Status           string
	Percent          string
	Msg              string
	Runtime          string
	Eta              string
	Split            string
	FinalStatusColor string
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

func footer(status, finalStatus string) string {
	var tpl bytes.Buffer
	var durString, etaString string

	if Options.ShowSummaryTimes {
		duration := time.Since(StartTime)
		durString = fmt.Sprintf(" Runtime[%s]", showDuration(duration))

		totalEta := time.Duration(TotalEtaSeconds) * time.Second
		remainingEta := time.Duration(totalEta.Seconds()-duration.Seconds()) * time.Second
		etaString = fmt.Sprintf(" ETA[%s]", showDuration(remainingEta))
	}

	// get a string with the summary line without a split gap (eta floats left)
	var ptpl bytes.Buffer
	percentValue := (float64(CompletedTasks) * float64(100)) / float64(TotalTasks)
	percent := Percent{percentValue}
	PercentTemplate.Execute(&ptpl, percent)
	percentStr := ptpl.String()

	SummaryTemplate.Execute(&tpl, Summary{status, percentStr, "", durString, etaString, "", ""})

	// calculate a space buffer to push the eta to the right
	terminalWidth, _ := terminal.Width()
	splitWidth := int(terminalWidth) - visualLength(tpl.String())
	if splitWidth < 0 {
		splitWidth = 0
	}

	tpl.Reset()
	SummaryTemplate.Execute(&tpl, Summary{status, percentStr, "", durString, etaString, strings.Repeat(" ", splitWidth), finalStatus})

	return tpl.String()
}

func Exists(name string) bool {
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
	for index := range conf.Tasks {
		failedTasks = append(failedTasks, conf.Tasks[index].process(index+1, len(conf.Tasks))...)

		if ExitSignaled {
			break
		}
	}

	err = Save(EtaCachePath, &CommandTimeCache)
	Check(err)

	var curLine int

	if Options.ShowSummaryFooter {
		if len(failedTasks) > 0 {
			display(footer(SummaryFailedArrow, FinalStatusError), &curLine, 0)
		} else {
			display(footer(SummarySuccessArrow, FinalStatusSuccess), &curLine, 0)
		}
	}

	if Options.ShowFailureReport {
		fmt.Println("Some tasks failed, see below for details.")
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
