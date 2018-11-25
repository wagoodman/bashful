package runtime

import (
	"github.com/wagoodman/bashful/config"
	"strings"
	"testing"
	"time"
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

//
// func TestSerialTaskEnvPersistence(t *testing.T) {
// 	var expStr, actStr string
// 	var failedTasks []*Task
// 	simpleYamlStr := `
// tasks:
//   - name: startNextSubTasks
//     cmd: export SOMEVAR=this
//
//   - name: append 'is'
//     cmd: export SOMEVAR=$SOMEVAR:is
//
//   - name: append 'DONTDOIT'
//     parallel-Tasks:
//       - cmd: export SOMEVAR=$SOMEVAR:DONTDOIT
//
//   - name: append '<replace>'
//     cmd: export SOMEVAR=$SOMEVAR:<replace>
//     for-each:
//       - working
//       - just
//
//   - name: append 'is'
//     cmd: eval 'export SOMEVAR=$SOMEVAR:fine'
// `
//
// 	environment := map[string]string{}
// 	config.Config.Options.StopOnFailure = false
// 	failedTasks = Execute([]byte(simpleYamlStr), environment)
// 	if len(failedTasks) > 0 {
// 		t.Error("TestSerialTaskEnvPersistence: Expected no Tasks to fail")
// 	}
//
// 	expStr, actStr = "this:is:working:just:fine", environment["SOMEVAR"]
// 	if expStr != actStr {
// 		t.Error("Expected", expStr, "got", actStr)
// 	}
//
// }

//
// func TestCurrentWorkingDirectory(t *testing.T) {
// 	var expStr, actStr string
// 	var failedTasks []*Task
// 	simpleYamlStr := `
// Tasks:
//   - name: startNextSubTasks
//     cmd: export CWD=$(pwd)
//     cwd: ../example
// `
//
// 	environment := map[string]string{}
// 	config.Config.Options.StopOnFailure = false
// 	failedTasks = Execute([]byte(simpleYamlStr), environment)
// 	if len(failedTasks) > 0 {
// 		t.Error("TestSerialTaskEnvPersistence: Expected no Tasks to fail")
// 	}
// 	cwd := strings.Split(environment["CWD"], "/")
// 	expStr, actStr = "example", cwd[len(cwd)-1]
// 	if expStr != actStr {
// 		t.Error("Expected", expStr, "got", actStr)
// 	}
//
// }
//
// func TestCommandArguments(t *testing.T) {
// 	yamlStr := `
// Tasks:
//   - cmd: command-with-args $1 $2
//     name: Arg1=$1 Arg2=$2
//   - cmd: all-args $*
//     name: Args=$*
// `
//
// 	config.Config.Cli.Args = []string{"First", "Second"}
// 	config.parseRunYaml([]byte(yamlStr))
// 	tasks := CreateTasks()
// 	if len(tasks) != 2 {
// 		t.Error("Expected two Tasks. Got: ", len(tasks))
// 	}
//
// 	if tasks[0].Config.CmdString != "command-with-args First Second" {
// 		t.Error("Expected arguments to be replaced. Got: ", tasks[0].Config.CmdString)
// 	}
// 	if tasks[0].Config.Name != "Arg1=First Arg2=Second" {
// 		t.Error("Expected arguments to be replaced in task name. Got: ", tasks[0].Config.Name)
// 	}
//
// 	if tasks[1].Config.CmdString != "all-args First Second" {
// 		t.Error("Expected all arguments to be replaced. Got: ", tasks[1].Config.CmdString)
// 	}
// 	if tasks[1].Config.Name != "Args=First Second" {
// 		t.Error("Expected all arguments to be replaced in task name. Got: ", tasks[1].Config.Name)
// 	}
// }
//
// func TestCreateTasks_SuccessfulParse(t *testing.T) {
// 	var expStr, actStr string
// 	var expOpt, actOpt bool
// 	var exNum int
// 	simpleYamlStr := `
// Tasks:
//   - cmd: random-worker.sh 10
//   - name: Compiling source
//     parallel-Tasks:
//       - cmd: compile-something.sh 2
//       - cmd: compile-something.sh 9
//         stop-on-failure: false
//         show-output: false
//       - cmd: compile-something.sh 6
//       - cmd: compile-something.sh 4 <replace>
//         for-each:
//           - plug 4
//           - plug 5
//           - plug 6
//   - name: some random task name <replace>
//     cmd: random-worker.sh 2
//     for-each:
//       - plug 3
//   - cmd: random-worker.sh 10 <replace>
//     for-each:
//       - plug 1
//       - plug 2
// `
// 	// load test time cache
// 	config.Config.cmdEtaCache = make(map[string]time.Duration)
// 	config.Config.cmdEtaCache["compile-something.sh 2"] = time.Duration(2 * time.Second)
// 	config.Config.cmdEtaCache["compile-something.sh 4"] = time.Duration(4 * time.Second)
// 	config.Config.cmdEtaCache["compile-something.sh 6"] = time.Duration(6 * time.Second)
// 	config.Config.cmdEtaCache["compile-something.sh 9"] = time.Duration(9 * time.Second)
// 	config.Config.cmdEtaCache["compile-something.sh 10"] = time.Duration(10 * time.Second)
//
// 	// load test Config yaml
// 	config.parseRunYaml([]byte(simpleYamlStr))
// 	// create and compile Tasks
// 	tasks := CreateTasks()
//
// 	// validate test task yaml
//
// 	exNum = 5
// 	if len(tasks) != exNum {
// 		t.Error("Expected", exNum, "Tasks got", len(tasks))
// 	}
//
// 	exNum = 6
// 	if len(tasks[1].Children) != exNum {
// 		t.Error("Expected", exNum, "parallel Tasks got", len(tasks[1].Children))
// 	}
//
// 	// ensure that names are set properly
//
// 	expStr, actStr = "Compiling source", tasks[1].Config.Name
// 	if actStr != expStr {
// 		t.Error("Expected name:", expStr, "got name:", actStr)
// 	}
//
// 	expStr, actStr = "some random task name plug 3", tasks[2].Config.Name
// 	if actStr != expStr {
// 		t.Error("Expected name:", expStr, "got name:", actStr)
// 	}
//
// 	// check the names of the top task list
//
// 	for _, taskIndex := range []int{0, 3, 4} {
// 		repr.Println(tasks[taskIndex].Config)
// 		expStr, actStr = tasks[taskIndex].Config.CmdString, tasks[taskIndex].Config.Name
// 		if actStr != expStr {
// 			t.Error("Expected name:", expStr, "got name:", actStr)
// 		}
// 	}
//
// 	// check the names of the top parallel list
// 	for taskIndex := 0; taskIndex < len(tasks[1].Children); taskIndex++ {
// 		expStr, actStr = tasks[1].Children[taskIndex].Config.CmdString, tasks[1].Children[taskIndex].Config.Name
// 		if actStr != expStr {
// 			t.Error("Expected name:", expStr, "got name:", actStr)
// 		}
// 	}
//
// 	// ensure that names and commands of for-each Tasks fill in the ? character with params
// 	for _, taskIndex := range []int{3, 4} {
// 		expStr, actStr = tasks[taskIndex].Config.CmdString, tasks[taskIndex].Config.Name
// 		if actStr != expStr {
// 			t.Error("Expected name:", expStr, "got name:", actStr)
// 		}
// 		if !strings.Contains(tasks[taskIndex].Config.Name, "plug") {
// 			t.Error("Expected name to contain 'plug' but got:", actStr)
// 		}
// 		if !strings.Contains(tasks[taskIndex].Config.CmdString, "plug") {
// 			t.Error("Expected cmd to contain 'plug' but got:", actStr)
// 		}
// 	}
//
// 	expStr, actStr = "some random task name plug 3", tasks[2].Config.Name
// 	if actStr != expStr {
// 		t.Error("Expected name:", expStr, "got name:", actStr)
// 	}
//
// 	expStr, actStr = "random-worker.sh 2", tasks[2].Config.CmdString
// 	if actStr != expStr {
// 		t.Error("Expected cmd:", expStr, "got cmd:", actStr)
// 	}
//
// 	// ensure stop on fail and show output can be overridden to false
// 	expOpt, actOpt = false, tasks[1].Children[1].Config.ShowTaskOutput
// 	if actOpt != expOpt {
// 		t.Error("Expected name:", expOpt, "got name:", actOpt)
// 	}
//
// 	expOpt, actOpt = false, tasks[1].Children[1].Config.StopOnFailure
// 	if actOpt != expOpt {
// 		t.Error("Expected name:", expOpt, "got name:", actOpt)
// 	}
//
// }
