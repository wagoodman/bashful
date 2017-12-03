package main

import (
	"sync/atomic"
	"sync"
	"fmt"

	terminal "github.com/wayneashleyberry/terminal-dimensions"
	"github.com/k0kubun/go-ansi"
)

var (
	instance *screen
	once     sync.Once
)

// Once is an object that will perform exactly one action.
type Once struct {
	m    sync.Mutex
	done uint32
}

// Do calls the function f if and only if Do is being called for the
// first time for this instance of Once. In other words, given
// 	var once Once
// if once.Do(f) is called multiple times, only the first call will invoke f,
// even if f has a different value in each invocation.  A new instance of
// Once is required for each function to execute.
//
// Do is intended for initialization that must be run exactly once.  Since f
// is niladic, it may be necessary to use a function literal to capture the
// arguments to a function to be invoked by Do:
// 	config.once.Do(func() { config.init(filename) })
//
// Because no call to Do returns until the one call to f returns, if f causes
// Do to be called, it will deadlock.
//
// If f panics, Do considers it to have returned; future calls of Do return
// without calling f.
//
func (o *Once) Do(f func()) {
	if atomic.LoadUint32(&o.done) == 1 {
		return
	}

	o.m.Lock()
	defer o.m.Unlock()
	if o.done == 0 {
		defer atomic.StoreUint32(&o.done, 1)
		f()
	}
}

type screen struct {
	numLines int
	curLine int
	hasHeader bool
	hasFooter bool
}

func Screen() *screen {
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
	for idx := 0 ; idx < numLines ; idx++ {
		scr.printLn("")
	}
	if hasFooter {
		scr.printLn("")
	}
	scr.MoveCursorToFirstLine()
}

func (scr *screen) MoveCursor(index int) {
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
	scr.MoveCursor(scr.numLines-1)
}

func (scr *screen) MoveCursorToHeader() {
	scr.MoveCursor(-1)
}

func (scr *screen) MoveCursorToFooter() {
	scr.MoveCursor(scr.numLines)
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

func (scr *screen) DisplayFooter(message string){
	scr.MoveCursorToFooter()
	scr.printLn(message)
}

func (scr *screen) DisplayHeader(message string){
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

func (scr *screen) MovePastFrame(keepFooter bool){
	scr.MoveCursorToFooter()
	if scr.hasFooter && keepFooter {
		ansi.CursorDown(1)
		scr.curLine++
	}
}

func (scr *screen) Display(message string, index int) {
	scr.MoveCursor(index)

	// trim message length if it won't fit on the screen
	terminalWidth, _ := terminal.Width()
	for visualLength(message) > int(terminalWidth) {
		message = trimToVisualLength(message, int(terminalWidth)-3) + "..."
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
