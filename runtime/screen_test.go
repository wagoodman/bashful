package runtime

import (
	"bytes"
	"io"
	"os"
	"testing"

	"github.com/alecthomas/repr"
	"github.com/wagoodman/bashful/utils"
)

func captureBoolStdout(f func(bool), x bool) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	f(x)

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	io.Copy(&buf, r)
	return buf.String()
}

func captureStrStdout(f func(string), x string) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	f(x)

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	io.Copy(&buf, r)
	return buf.String()
}

func captureIntStdout(f func(int), x int) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	f(x)

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	io.Copy(&buf, r)
	return buf.String()
}

func captureStrIntStdout(f func(string, int), x string, y int) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	f(x, y)

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	io.Copy(&buf, r)
	return buf.String()
}

func TestVisualLength(t *testing.T) {
	ansiString := "\x1b[2mHello, World!\x1b[0m"
	normalLen := len(ansiString)
	if normalLen != 21 {
		t.Error("TestVisualLength: Test harness not working! Expected 15 got", normalLen)
	}

	visualLen := utils.VisualLength(ansiString)
	if visualLen != 13 {
		t.Error("Expected 13 got", visualLen)
	}
}

func TestTrimToVisualLength(t *testing.T) {
	normalString := "Hello, World!"
	ansiString := "\x1b[2mHel\x1b[3mlo, Wor\x1b[0mld!\x1b[0m"

	for idx := 0; idx < len(normalString); idx++ {
		trimString := utils.TrimToVisualLength(ansiString, idx)
		if utils.VisualLength(trimString) != idx {
			t.Error("TestTrimToVisualLength: Expected", idx, "got", utils.VisualLength(trimString), ". Trim:", trimString)
		}
	}
}

func TestMoveCursor(t *testing.T) {
	var expectedOutput, testOutput string
	scr := NewScreen()
	scr.ResetFrame(5, false, false)

	// stay in place
	scr.curLine = 1
	expectedOutput = ""
	testOutput = captureIntStdout(scr.MoveCursor, 1)
	if expectedOutput != testOutput {
		t.Error("TestMoveCursor (Stay in place): Expected", repr.String(expectedOutput), "got", repr.String(testOutput))
	}

	// move to the next line
	scr.curLine = 1
	expectedOutput = "\x1b[1B"
	testOutput = captureIntStdout(scr.MoveCursor, 2)
	if expectedOutput != testOutput {
		t.Error("TestMoveCursor (Move to the next line): Expected", repr.String(expectedOutput), "got", repr.String(testOutput))
	}

	// move a few lines away
	scr.curLine = 1
	expectedOutput = "\x1b[2B"
	testOutput = captureIntStdout(scr.MoveCursor, 3)
	if expectedOutput != testOutput {
		t.Error("TestMoveCursor (Move a few lines away): Expected", repr.String(expectedOutput), "got", repr.String(testOutput))
	}

	// move past the frame (no footer, to last line)
	scr.curLine = 1
	expectedOutput = "\x1b[3B"
	testOutput = captureIntStdout(scr.MoveCursor, 10)
	if expectedOutput != testOutput {
		t.Error("TestMoveCursor (Move past the frame, to last line): Expected", repr.String(expectedOutput), "got", repr.String(testOutput))
	}

	// move past the frame (no header, to first line)
	scr.curLine = 2
	expectedOutput = "\x1b[2A"
	testOutput = captureIntStdout(scr.MoveCursor, -10)
	if expectedOutput != testOutput {
		t.Error("TestMoveCursor (Move past the frame, to first line): Expected", repr.String(expectedOutput), "got", repr.String(testOutput))
	}

	scr.ResetFrame(5, true, true)

	// move past the frame (to footer)
	scr.curLine = 1
	expectedOutput = "\x1b[4B"
	testOutput = captureIntStdout(scr.MoveCursor, 10)
	if expectedOutput != testOutput {
		t.Error("TestMoveCursor (Move past the frame, to footer): Expected", repr.String(expectedOutput), "got", repr.String(testOutput))
	}

	// move past the frame (to header)
	scr.curLine = 2
	expectedOutput = "\x1b[3A"
	testOutput = captureIntStdout(scr.MoveCursor, -10)
	if expectedOutput != testOutput {
		t.Error("TestMoveCursor (Move past the frame, to header): Expected", repr.String(expectedOutput), "got", repr.String(testOutput))
	}

}

func TestMovePastFrame(t *testing.T) {
	var testOutput string
	scr := NewScreen()
	scr.ResetFrame(5, false, false)

	var testData = []struct {
		hasHeader      bool
		hasFooter      bool
		keepHeader     bool
		expectedOutput string
	}{
		{false, false, false, "\x1b[3B\x1b[1B"},
		{false, true, false, "\x1b[4B"},
		{true, false, false, "\x1b[3B\x1b[1B"},
		{true, true, false, "\x1b[4B"},
		{false, false, true, "\x1b[3B\x1b[1B"},
		{false, true, true, "\x1b[4B\x1b[1B"},
		{true, false, true, "\x1b[3B\x1b[1B"},
		{true, true, true, "\x1b[4B\x1b[1B"},
	}

	for _, testObj := range testData {
		scr.ResetFrame(5, testObj.hasHeader, testObj.hasFooter)
		scr.curLine = 1
		testOutput = captureBoolStdout(scr.MovePastFrame, testObj.keepHeader)
		if testObj.expectedOutput != testOutput {
			t.Error("MovePastFrame (header:", testObj.hasHeader, ", footer:", testObj.hasFooter, ", keepheader:", testObj.keepHeader, "): Expected", repr.String(testObj.expectedOutput), "got", repr.String(testOutput))
		}
	}
}

func TestDisplayFooter(t *testing.T) {
	var expectedOutput, testOutput string
	scr := NewScreen()
	scr.ResetFrame(5, false, false)

	scr.curLine = 1
	expectedOutput = "\x1b[3B\x1b[2K\x1b[0GThis is the footer.\n"
	testOutput = captureStrStdout(scr.DisplayFooter, "This is the footer.")

	if expectedOutput != testOutput {
		t.Error("TestDisplayFooter (Stay in place): Expected", repr.String(expectedOutput), "got", repr.String(testOutput))
	}

}

func TestDisplayHeader(t *testing.T) {
	var expectedOutput, testOutput string
	scr := NewScreen()
	scr.ResetFrame(5, false, false)

	scr.curLine = 1
	expectedOutput = "\x1b[1A\x1b[2K\x1b[0GThis is the header.\n"
	testOutput = captureStrStdout(scr.DisplayHeader, "This is the header.")

	if expectedOutput != testOutput {
		t.Error("TestDisplayFooter (Stay in place): Expected", repr.String(expectedOutput), "got", repr.String(testOutput))
	}

}

func TestPrintLn(t *testing.T) {
	var expectedOutput, testOutput string
	scr := NewScreen()
	scr.ResetFrame(5, false, false)

	scr.curLine = 1
	expectedOutput = "\x1b[2K\x1b[0GHello, World!\n"
	testOutput = captureStrStdout(scr.printLn, "Hello, World!")
	if expectedOutput != testOutput {
		t.Error("TestPrintLn (Stay in place): Expected", repr.String(expectedOutput), "got", repr.String(testOutput))
	}

}

func TestDisplay(t *testing.T) {
	var expectedOutput, testOutput string
	scr := NewScreen()
	scr.ResetFrame(5, false, false)

	terminalWidth = func() (uint, error) {
		return uint(20), nil
	}

	scr.curLine = 1
	expectedOutput = "\x1b[2B\x1b[2K\x1b[0GHello, World!\n"
	testOutput = captureStrIntStdout(scr.Display, "Hello, World!", 3)
	if expectedOutput != testOutput {
		t.Error("TestDisplay (simply display): Expected", repr.String(expectedOutput), "got", repr.String(testOutput))
	}

	scr.curLine = 1
	expectedOutput = "\x1b[2B\x1b[2K\x1b[0GHello, World!!!!!...\n"
	testOutput = captureStrIntStdout(scr.Display, "Hello, World!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!", 3)
	if expectedOutput != testOutput {
		t.Error("TestDisplay (trim long lines): Expected", repr.String(expectedOutput), "got", repr.String(testOutput))
	}

}
