// Copyright © 2018 Alex Goodman
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package runtime

import (
	"bytes"
	"time"
	"fmt"
	"github.com/wagoodman/bashful/utils"
	color "github.com/mgutz/ansi"
	"strings"
	"github.com/wagoodman/bashful/config"
	terminal "github.com/wayneashleyberry/terminal-dimensions"

	"strconv"
)

var (
	purple             = color.ColorFunc("magenta+h")
	red                = color.ColorFunc("red+h")
	blue               = color.ColorFunc("blue+h")
	Bold               = color.ColorFunc("default+b")
)

// Color returns the ansi color value represented by the given status
func (status status) Color(attributes string) string {
	switch status {
	case statusRunning:
		return color.ColorCode(strconv.Itoa(config.Config.Options.ColorRunning) + "+" + attributes)

	case statusPending:
		return color.ColorCode(strconv.Itoa(config.Config.Options.ColorPending) + "+" + attributes)

	case StatusSuccess:
		return color.ColorCode(strconv.Itoa(config.Config.Options.ColorSuccess) + "+" + attributes)

	case StatusError:
		return color.ColorCode(strconv.Itoa(config.Config.Options.ColorError) + "+" + attributes)

	}
	return "INVALID COMMAND STATUS"
}


func footer(status status, message string, invoker *Executor) string {
	var tpl bytes.Buffer
	var durString, etaString, stepString, errorString string

	if config.Config.Options.ShowSummaryTimes {
		duration := time.Since(StartTime)
		durString = fmt.Sprintf(" Runtime[%s]", utils.ShowDuration(duration))

		totalEta := time.Duration(config.Config.TotalEtaSeconds) * time.Second
		remainingEta := time.Duration(totalEta.Seconds()-duration.Seconds()) * time.Second
		etaString = fmt.Sprintf(" ETA[%s]", utils.ShowDuration(remainingEta))
	}

	if len(invoker.CompletedTasks) == invoker.TotalTasks {
		etaString = ""
	}

	if config.Config.Options.ShowSummarySteps {
		stepString = fmt.Sprintf(" Tasks[%d/%d]", len(invoker.CompletedTasks), invoker.TotalTasks)
	}

	if config.Config.Options.ShowSummaryErrors {
		errorString = fmt.Sprintf(" Errors[%d]", len(invoker.FailedTasks))
	}

	// get a string with the summary line without a split gap (eta floats left)
	percentValue := (float64(len(invoker.CompletedTasks)) * float64(100)) / float64(invoker.TotalTasks)
	percentStr := fmt.Sprintf("%3.2f%% Complete", percentValue)

	if len(invoker.CompletedTasks) == invoker.TotalTasks {
		percentStr = status.Color("b") + percentStr + color.Reset
	} else {
		percentStr = color.Color(percentStr, "default+b")
	}

	summaryTemplate.Execute(&tpl, summary{Status: status.Color("i"), Percent: percentStr, Runtime: durString, Eta: etaString, Steps: stepString, Errors: errorString, Msg: message})

	// calculate a space buffer to push the eta to the right
	terminalWidth, _ := terminal.Width()
	splitWidth := int(terminalWidth) - utils.VisualLength(tpl.String())
	if splitWidth < 0 {
		splitWidth = 0
	}

	tpl.Reset()
	summaryTemplate.Execute(&tpl, summary{Status: status.Color("i"), Percent: percentStr, Runtime: Bold(durString), Eta: Bold(etaString), Split: strings.Repeat(" ", splitWidth), Steps: Bold(stepString), Errors: Bold(errorString), Msg: message})

	return tpl.String()
}

// CurrentEta returns a formatted string indicating a countdown until command completion
func (task *Task) CurrentEta() string {
	var eta, etaValue string

	if config.Config.Options.ShowTaskEta {
		running := time.Since(task.Command.StartTime)
		etaValue = "Unknown!"
		if task.Command.EstimatedRuntime > 0 {
			etaValue = utils.ShowDuration(time.Duration(task.Command.EstimatedRuntime.Seconds()-running.Seconds()) * time.Second)
		}
		eta = fmt.Sprintf(Bold("[%s]"), etaValue)
	}
	return eta
}
