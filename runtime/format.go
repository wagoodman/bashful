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
	"time"
	"fmt"
	"github.com/wagoodman/bashful/utils"
	color "github.com/mgutz/ansi"
	"github.com/wagoodman/bashful/config"
	"strconv"
)

// Color returns the ansi color value represented by the given TaskStatus
func (status TaskStatus) Color(attributes string) string {
	switch status {
	case StatusRunning:
		return color.ColorCode(strconv.Itoa(config.Config.Options.ColorRunning) + "+" + attributes)

	case StatusPending:
		return color.ColorCode(strconv.Itoa(config.Config.Options.ColorPending) + "+" + attributes)

	case StatusSuccess:
		return color.ColorCode(strconv.Itoa(config.Config.Options.ColorSuccess) + "+" + attributes)

	case StatusError:
		return color.ColorCode(strconv.Itoa(config.Config.Options.ColorError) + "+" + attributes)

	}
	return "INVALID COMMAND STATUS"
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
		eta = fmt.Sprintf(utils.Bold("[%s]"), etaValue)
	}
	return eta
}
