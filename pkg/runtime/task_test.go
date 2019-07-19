package runtime

import (
	"strings"
	"testing"
	"time"

	"github.com/lunixbochs/vtclean"
	"github.com/wagoodman/bashful/pkg/config"
)

func Test_Task_estimateRuntime(t *testing.T) {
	table := map[string]struct {
		index         int
		maxParallel   int
		parentTaskEta int
		childTaskEta  []int
		expectedEta   float64
		runYaml       []byte
	}{

		"single task": {
			index:         0,
			maxParallel:   4,
			parentTaskEta: 20,
			childTaskEta:  []int{},
			expectedEta:   20,
			runYaml: []byte(`
tasks:
  - cmd: ./do/thing.sh 4`),
		},

		"parallel tasks": {
			index:         0,
			maxParallel:   4,
			parentTaskEta: 0,
			childTaskEta:  []int{20, 30, 40},
			expectedEta:   40,
			runYaml: []byte(`
tasks:
  - parallel-tasks:
    - cmd: ./do/thing.sh 2
    - cmd: ./do/thing.sh 3
    - cmd: ./do/thing.sh 4`),
		},

		"parallel tasks, restricted concurrency": {
			index:         0,
			maxParallel:   2,
			parentTaskEta: 0,
			childTaskEta:  []int{20, 30, 40},
			expectedEta:   60,
			runYaml: []byte(`
tasks:
  - parallel-tasks:
      - cmd: ./do/thing.sh 2
      - cmd: ./do/thing.sh 3
      - cmd: ./do/thing.sh 4`),
		},
	}

	for name, testCase := range table {
		t.Logf("Running test case: %s", name)
		client, err := NewClientFromYaml(testCase.runYaml, &config.Cli{})
		if err != nil {
			t.Errorf("unexpeced error: %v", err)
		}

		// load test data...
		client.Config.Options.MaxParallelCmds = testCase.maxParallel
		for _, task := range client.Executor.Tasks {
			task.Command.addEstimatedRuntime(time.Duration(testCase.parentTaskEta) * time.Second)
			for cIdx, subTask := range task.Children {
				subTask.Command.addEstimatedRuntime(time.Duration(testCase.childTaskEta[cIdx]) * time.Second)
			}
		}

		// assert...
		task := client.Executor.Tasks[testCase.index]

		runtime := task.estimateRuntime()
		if runtime != testCase.expectedEta {
			t.Errorf("   expected eta='%v', got '%v'", testCase.expectedEta, runtime)
		}
	}
}

func Test_Task_UpdateExec(t *testing.T) {
	runYaml := []byte(`
tasks:
  - cmd: <exec> run
    url: https://some-url.com/the-script.sh
  - url: https://some-url.com/the-other-script.sh`)

	client, err := NewClientFromYaml(runYaml, &config.Cli{})
	if err != nil {
		t.Errorf("unexpeced error: %v", err)
	}

	table := map[string]struct {
		index       int
		expectedUrl string
		expectedCmd string
	}{
		"command template": {
			index:       0,
			expectedUrl: "https://some-url.com/the-script.sh",
			expectedCmd: "/a/path/to/a/tempfile.sh run",
		},
		"empty template": {
			index:       1,
			expectedUrl: "https://some-url.com/the-other-script.sh",
			expectedCmd: "/a/path/to/a/tempfile.sh",
		},
	}

	for name, testCase := range table {
		t.Logf("Running test case: %s", name)

		task := client.Executor.Tasks[testCase.index]
		task.UpdateExec("/a/path/to/a/tempfile.sh")

		if task.Config.CmdString != testCase.expectedCmd {
			t.Errorf("   expected cmdString='%s', got '%s'", testCase.expectedCmd, task.Config.CmdString)
		}
		if task.Config.URL != testCase.expectedUrl {
			t.Errorf("   expected url='%s', got '%s'", testCase.expectedUrl, task.Config.URL)
		}

		// the real command may be a bit more verbose, only check if the command is in the final cmd obj string
		cmdObjString := strings.Join(task.Command.Cmd.Args, " ")
		if !strings.Contains(cmdObjString, task.Config.CmdString) {
			t.Errorf("   expected cmd='%+v', got '%+v'", testCase.expectedCmd, cmdObjString)
		}
	}
}

func Test_Task_requiresSudoPassword(t *testing.T) {
	var table = map[string]struct {
		taskConfig     config.TaskConfig
		expectedResult bool
	}{
		"parallel with sudo child": {
			taskConfig: config.TaskConfig{
				Name: "parent-task",
				ParallelTasks: []config.TaskConfig{
					{
						Name:      "first-child",
						CmdString: "./bin/first-thing.sh",
					},
					{
						Name:      "second-child",
						CmdString: "./bin/second-thing.sh",
						Sudo:      true,
					},
				},
			},
			expectedResult: true,
		},
		"parallel no sudo": {
			taskConfig: config.TaskConfig{
				Name: "parent-task",
				ParallelTasks: []config.TaskConfig{
					{
						Name:      "first-child",
						CmdString: "./bin/first-thing.sh",
					},
					{
						Name:      "second-child",
						CmdString: "./bin/second-thing.sh",
					},
				},
			},
			expectedResult: false,
		},
		"parallel with sudo parent": {
			taskConfig: config.TaskConfig{
				Name: "parent-task",
				Sudo: true,
				ParallelTasks: []config.TaskConfig{
					{
						Name:      "first-child",
						CmdString: "./bin/first-thing.sh",
					},
					{
						Name:      "second-child",
						CmdString: "./bin/second-thing.sh",
					},
				},
			},
			expectedResult: false,
		},
		"single task with sudo": {
			taskConfig: config.TaskConfig{
				Name:      "parent-task",
				CmdString: "./bin/parent-thing.sh",
				Sudo:      true,
			},
			expectedResult: true,
		},
	}

	for name, testCase := range table {
		t.Logf("Running test case: %s", name)
		task := NewTask(testCase.taskConfig, nil)
		if task.requiresSudoPassword() != testCase.expectedResult {
			t.Errorf("expected requiresSudoPassword='%v'", testCase.expectedResult)
		}
	}
}

func Test_Task_Execute(t *testing.T) {
	var table = map[string]struct {
		taskConfig     config.TaskConfig
		expectedEvents []TaskEvent
		expectedEnv    map[string]string
	}{
		"single task (success)": {
			taskConfig: config.TaskConfig{
				Name:      "easy task",
				CmdString: "true",
			},
			expectedEvents: []TaskEvent{
				{nil, StatusRunning, "", "", false, -1},
				{nil, StatusSuccess, "", "", true, 0},
			},
			expectedEnv: map[string]string{},
		},
		"single task (failure)": {
			taskConfig: config.TaskConfig{
				Name:      "easy task",
				CmdString: "false",
			},
			expectedEvents: []TaskEvent{
				{nil, StatusRunning, "", "", false, -1},
				{nil, StatusError, "", "", true, 1},
			},
			expectedEnv: map[string]string{},
		},
		"single task (ignore failure)": {
			taskConfig: config.TaskConfig{
				Name:          "easy task",
				CmdString:     "false",
				IgnoreFailure: true,
			},
			expectedEvents: []TaskEvent{
				{nil, StatusRunning, "", "", false, -1},
				{nil, StatusSuccess, "", "", true, 1},
			},
			expectedEnv: map[string]string{},
		},
		"single task (env persist)": {
			taskConfig: config.TaskConfig{
				Name:          "easy task",
				CmdString:     "export ANSWER=42",
				IgnoreFailure: true,
			},
			expectedEvents: []TaskEvent{
				{nil, StatusRunning, "", "", false, -1},
				{nil, StatusSuccess, "", "", true, 0},
			},
			expectedEnv: map[string]string{
				"INITIAL_TEST_DATA": "ALSO42",
				"ANSWER":            "42",
			},
		},
		"single task (stdout)": {
			taskConfig: config.TaskConfig{
				Name:        "easy task",
				CmdString:   "echo sup",
				EventDriven: true,
			},
			expectedEvents: []TaskEvent{
				{nil, StatusRunning, "", "", false, -1},
				{nil, StatusRunning, "sup", "", false, -1},
				{nil, StatusSuccess, "", "", true, 0},
			},
			expectedEnv: map[string]string{},
		},
		"single task (stderr)": {
			taskConfig: config.TaskConfig{
				Name:        "easy task",
				CmdString:   "echo meh >>/dev/stderr",
				EventDriven: true,
			},
			expectedEvents: []TaskEvent{
				{nil, StatusRunning, "", "", false, -1},
				{nil, StatusRunning, "", "meh", false, -1},
				{nil, StatusSuccess, "", "", true, 0},
			},
			expectedEnv: map[string]string{},
		},
	}

	for name, testCase := range table {
		t.Logf("--- Running test case: %s", name)
		task := NewTask(testCase.taskConfig, nil)

		environment := map[string]string{
			"INITIAL_TEST_DATA": "ALSO42",
		}

		events := make([]TaskEvent, 0)
		go task.Execute(task.events, &task.waiter, environment)

		for event := range task.events {
			t.Logf("  recording event %d (%s): %v", len(events), event.Task.Config.Name, event)
			events = append(events, event)
			if event.Complete {
				close(task.events)
			}
		}

		if len(events) != len(testCase.expectedEvents) {
			t.Fatalf("expected %v events, got %v", len(testCase.expectedEvents), len(events))
		}

		for idx, expEvent := range testCase.expectedEvents {
			actualEvent := events[idx]
			if expEvent.Status != actualEvent.Status {
				t.Errorf("   event %d: expected status=%v, got %v", idx, expEvent.Status, actualEvent.Status)
			}
			if expEvent.Complete != actualEvent.Complete {
				t.Errorf("   event %d: expected complete=%v, got %v", idx, expEvent.Complete, actualEvent.Complete)
			}
			if expEvent.ReturnCode != actualEvent.ReturnCode {
				t.Errorf("   event %d: expected rc=%v, got %v", idx, expEvent.ReturnCode, actualEvent.ReturnCode)
			}
			if expEvent.Stderr != vtclean.Clean(actualEvent.Stderr, false) {
				t.Errorf("   event %d: expected stderr='%v', got '%v'", idx, expEvent.Stderr, actualEvent.Stderr)
			}
			if expEvent.Stdout != vtclean.Clean(actualEvent.Stdout, false) {
				t.Errorf("   event %d: expected stdout='%v', got '%v'", idx, expEvent.Stdout, actualEvent.Stdout)
			}

		}

		// this is not valid, there may be several variables generated, few of which are intentional
		// if len(environment) != len(testCase.expectedEnv) {
		// 	for key, actualValue := range environment {
		// 		t.Errorf("   environment[%s] = %s", key, actualValue)
		// 	}
		// 	t.Fatalf("expected %v environment variables, got %v", len(testCase.expectedEnv), len(environment))
		// }

		for key, expValue := range testCase.expectedEnv {

			if actualValue, exists := environment[key]; exists {
				if expValue != actualValue {
					t.Errorf("   expected env[%s]=%v, got %v", key, expValue, actualValue)
				}
			} else {
				t.Errorf("   missing environment key '%s'", key)
			}

		}

	}
}
