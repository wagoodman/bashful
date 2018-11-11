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

package core

import (
	"fmt"
	"sync"

	"github.com/k0kubun/go-ansi"
	terminal "github.com/wayneashleyberry/terminal-dimensions"
	"github.com/wagoodman/bashful/utils"
	"github.com/wagoodman/bashful/log"
)

var (
	instance      *screen
	once          sync.Once
	terminalWidth = terminal.Width
)

type screen struct {
	numLines  int
	curLine   int
	hasHeader bool
	hasFooter bool
}

// NewScreen is a singleton that represents the screen frame being actively written to
func NewScreen() *screen {
	once.Do(func() {
		instance = &screen{}
	})
	return instance
}

func (scr *screen) ResetFrame(numLines int, hasHeader, hasFooter bool) {
	scr.curLine = 0
	scr.numLines = numLines
	scr.hasFooter = hasFooter
	scr.hasHeader = hasHeader

	if hasHeader {
		// note: this index doesn't count!
		fmt.Println("")
	}
	for idx := 0; idx < numLines; idx++ {
		scr.printLn("")
	}
	if hasFooter {
		scr.printLn("")
	}
	scr.MoveCursorToFirstLine()
}

func (scr *screen) MoveCursor(index int) {
	// move to the first possible line (first line or header) if asked to move beyond defined frame
	if index < 0 && !scr.hasHeader {
		index = 0
	}
	if index < -1 && scr.hasHeader {
		index = -1
	}
	// move to the last possible line (last line or footer) if asked to move beyond defined frame
	if index > scr.numLines-1 && !scr.hasFooter {
		index = scr.numLines - 1
	}
	if index > scr.numLines && scr.hasFooter {
		index = scr.numLines
	}

	moves := scr.curLine - index
	if moves != 0 {
		if moves < 0 {
			ansi.CursorDown(moves * -1)
		} else {
			ansi.CursorUp(moves)
		}
		scr.curLine -= moves
	}
}

func (scr *screen) MoveCursorToFirstLine() {
	scr.MoveCursor(0)
}

func (scr *screen) MoveCursorToLastLine() {
	scr.MoveCursor(scr.numLines - 1)
}

func (scr *screen) MoveCursorToHeader() {
	scr.MoveCursor(-1)
}

func (scr *screen) MoveCursorToFooter() {
	scr.MoveCursor(scr.numLines)
}

func (scr *screen) DisplayFooter(message string) {
	scr.MoveCursorToFooter()
	scr.printLn(message)
}

func (scr *screen) DisplayHeader(message string) {
	scr.MoveCursorToHeader()
	scr.printLn(message)
}

func (scr *screen) EraseBelowHeader() {
	// erase from the first to the last line
	scr.MoveCursorToFirstLine()
	for idx := 0; idx < scr.numLines; idx++ {
		scr.printLn("")
	}
	// additionally delete footer
	if scr.hasFooter {
		scr.DisplayFooter("")
	}
}

func (scr *screen) MovePastFrame(keepFooter bool) {
	scr.MoveCursorToFooter()
	if scr.hasFooter && keepFooter || !scr.hasFooter {
		ansi.CursorDown(1)
		scr.curLine++
	}
}

func (scr *screen) Display(message string, index int) {
	scr.MoveCursor(index)

	// trim message length if it won't fit on the screen
	width, err := terminalWidth()
	if err != nil {
		log.LogToMain("Unable to determine screen width", errorFormat)
		width = 80
	}
	for utils.VisualLength(message) > int(width) {
		message = utils.TrimToVisualLength(message, int(width)-3) + "..."
	}

	scr.printLn(message)
}

func (scr *screen) printLn(message string) {
	ansi.EraseInLine(2)
	ansi.CursorHorizontalAbsolute(0)
	// note: ansi cursor down cannot be used as this may be the last row
	fmt.Println(message)
	scr.curLine++
}
