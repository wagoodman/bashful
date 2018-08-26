package core

import (
	"fmt"
	"sync"

	"github.com/k0kubun/go-ansi"
	terminal "github.com/wayneashleyberry/terminal-dimensions"
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

// newScreen is a singleton that represents the screen frame being actively written to
func newScreen() *screen {
	once.Do(func() {
		instance = &screen{}
	})
	return instance
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
	for visualLength(message) > length && len(message) > 1 {
		message = message[:len(message)-1]
	}
	return message
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
		LogToMain("Unable to determine screen width", errorFormat)
		width = 80
	}
	for visualLength(message) > int(width) {
		message = trimToVisualLength(message, int(width)-3) + "..."
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
