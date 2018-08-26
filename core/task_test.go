package core

import (
	"strings"
	"testing"

	"github.com/alecthomas/repr"
)

func TestTaskString(t *testing.T) {
	var testOutput, expectedOutput string

	taskConfig := TaskConfig{
		Name:      "some name!",
		CmdString: "/bin/true",
	}
	task := NewTask(taskConfig, 1, "2")
	task.Display.Values = LineInfo{Status: statusSuccess.Color(""), Title: task.Config.Name, Msg: "some message", Prefix: "$", Eta: "SOMEETAVALUE"}

	testOutput = task.String(50)
	expectedOutput = " \x1b[38;5;10m  \x1b[0m • some name!                som...SOMEETAVALUE"
	if expectedOutput != testOutput {
		t.Error("TestTaskString (default): Expected", repr.String(expectedOutput), "got", repr.String(testOutput))
	}

	Config.Options.ShowTaskEta = false
	task.Display.Values.Title = "123456789qwertyuiopasdfghjklzxcvbnm234567890qwertyuiopasdfghjklzxcvbnm123456789qwertyuiopasdfghjklzxcvbnm234567890qwertyuiopasdfghjklzxcvbnm"
	testOutput = task.String(20)
	expectedOutput = " \x1b[38;5;10m  \x1b[0m • 123456789qwertyuiopasdfghjklzxcvbnm234567890qwertyuiopasdfghjklzxcvbnm123456789qwertyuiopasdfghjklzxcvbnm234567890qwertyuiopasdfghjklzxcvbnm s...SOMEETAVALUE"
	if expectedOutput != testOutput {
		t.Error("TestTaskString (eta, truncate): Expected", repr.String(expectedOutput), "got", repr.String(testOutput))
	}

}

func TestSerialTaskEnvPersistence(t *testing.T) {
	var expStr, actStr string
	var failedTasks []*Task
	simpleYamlStr := `
tasks:
  - name: start
    cmd: export SOMEVAR=this 

  - name: append 'is'
    cmd: export SOMEVAR=$SOMEVAR:is

  - name: append 'DONTDOIT'
    parallel-tasks:
      - cmd: export SOMEVAR=$SOMEVAR:DONTDOIT

  - name: append '<replace>'
    cmd: export SOMEVAR=$SOMEVAR:<replace>
    for-each:
      - working
      - just

  - name: append 'is'
    cmd: eval 'export SOMEVAR=$SOMEVAR:fine'
`

	environment := map[string]string{}
	Config.Options.StopOnFailure = false
	failedTasks = Run([]byte(simpleYamlStr), environment)
	if len(failedTasks) > 0 {
		t.Error("TestSerialTaskEnvPersistence: Expected no tasks to fail")
	}

	expStr, actStr = "this:is:working:just:fine", environment["SOMEVAR"]
	if expStr != actStr {
		t.Error("Expected", expStr, "got", actStr)
	}

}

func TestCurrentWorkingDirectory(t *testing.T) {
	var expStr, actStr string
	var failedTasks []*Task
	simpleYamlStr := `
tasks:
  - name: start
    cmd: export CWD=$(pwd)
    cwd: ../example
`

	environment := map[string]string{}
	Config.Options.StopOnFailure = false
	failedTasks = Run([]byte(simpleYamlStr), environment)
	if len(failedTasks) > 0 {
		t.Error("TestSerialTaskEnvPersistence: Expected no tasks to fail")
	}
	cwd := strings.Split(environment["CWD"], "/")
	expStr, actStr = "example", cwd[len(cwd)-1]
	if expStr != actStr {
		t.Error("Expected", expStr, "got", actStr)
	}

}
