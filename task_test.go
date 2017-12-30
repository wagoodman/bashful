package main

import (
	"bytes"
	"io"
	"os"
	"testing"

	"github.com/alecthomas/repr"
)

func captureStdout(f func()) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	f()

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	io.Copy(&buf, r)
	return buf.String()
}

// TODO: rewrite this
func TestTaskString(t *testing.T) {
	var testOutput, expectedOutput string

	taskConfig := TaskConfig{
		Name:      "some name!",
		CmdString: "/bin/true",
	}
	task := NewTask(taskConfig, 1, "2")
	task.Display.Values = LineInfo{Status: StatusSuccess.Color("i"), Title: task.Config.Name, Msg: "some message", Spinner: "$", Eta: "SOMEETAVALUE"}

	testOutput = task.String(50)
	expectedOutput = " \x1b[7;92m  \x1b[0m • some name!                som...SOMEETAVALUE"
	if expectedOutput != testOutput {
		t.Error("TestTaskString (default): Expected", repr.String(expectedOutput), "got", repr.String(testOutput))
	}

	config.Options.ShowTaskEta = false
	task.Display.Values.Title = "123456789qwertyuiopasdfghjklzxcvbnm234567890qwertyuiopasdfghjklzxcvbnm123456789qwertyuiopasdfghjklzxcvbnm234567890qwertyuiopasdfghjklzxcvbnm"
	testOutput = task.String(20)
	expectedOutput = " \x1b[7;92m  \x1b[0m • 123456789qwertyuiopasdfghjklzxcvbnm234567890qwertyuiopasdfghjklzxcvbnm123456789qwertyuiopasdfghjklzxcvbnm234567890qwertyuiopasdfghjklzxcvbnm s...SOMEETAVALUE"
	if expectedOutput != testOutput {
		t.Error("TestTaskString (eta, truncate): Expected", repr.String(expectedOutput), "got", repr.String(testOutput))
	}

}
