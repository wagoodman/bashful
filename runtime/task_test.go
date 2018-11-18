package runtime

//
// import (
// 	"strings"
// 	"testing"
//
// 	"github.com/alecthomas/repr"
// 	"github.com/wagoodman/bashful/config"
// 	"time"
// )
//
// func TestTaskString(t *testing.T) {
// 	var testOutput, expectedOutput string
//
// 	taskConfig := config.TaskConfig{
// 		Name:      "some name!",
// 		CmdString: "/bin/true",
// 	}
// 	task := NewTask(taskConfig, 1, "2")
// 	task.Display.Values = lineInfo{Status: StatusSuccess.Color(""), Title: task.Config.Name, Msg: "some message", Prefix: "$", Eta: "SOMEETAVALUE"}
//
// 	testOutput = task.String(50)
// 	expectedOutput = " \x1b[38;5;10m  \x1b[0m • some name!                som...SOMEETAVALUE"
// 	if expectedOutput != testOutput {
// 		t.Error("TestTaskString (default): Expected", repr.String(expectedOutput), "got", repr.String(testOutput))
// 	}
//
// 	config.Config.Options.ShowTaskEta = false
// 	task.Display.Values.Title = "123456789qwertyuiopasdfghjklzxcvbnm234567890qwertyuiopasdfghjklzxcvbnm123456789qwertyuiopasdfghjklzxcvbnm234567890qwertyuiopasdfghjklzxcvbnm"
// 	testOutput = task.String(20)
// 	expectedOutput = " \x1b[38;5;10m  \x1b[0m • 123456789qwertyuiopasdfghjklzxcvbnm234567890qwertyuiopasdfghjklzxcvbnm123456789qwertyuiopasdfghjklzxcvbnm234567890qwertyuiopasdfghjklzxcvbnm s...SOMEETAVALUE"
// 	if expectedOutput != testOutput {
// 		t.Error("TestTaskString (eta, truncate): Expected", repr.String(expectedOutput), "got", repr.String(testOutput))
// 	}
//
// }
//
// func TestSerialTaskEnvPersistence(t *testing.T) {
// 	var expStr, actStr string
// 	var failedTasks []*Task
// 	simpleYamlStr := `
// Tasks:
//   - name: start
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
// 	failedTasks = Run([]byte(simpleYamlStr), environment)
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
//   - name: start
//     cmd: export CWD=$(pwd)
//     cwd: ../example
// `
//
// 	environment := map[string]string{}
// 	config.Config.Options.StopOnFailure = false
// 	failedTasks = Run([]byte(simpleYamlStr), environment)
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
// 	config.Config.CommandTimeCache = make(map[string]time.Duration)
// 	config.Config.CommandTimeCache["compile-something.sh 2"] = time.Duration(2 * time.Second)
// 	config.Config.CommandTimeCache["compile-something.sh 4"] = time.Duration(4 * time.Second)
// 	config.Config.CommandTimeCache["compile-something.sh 6"] = time.Duration(6 * time.Second)
// 	config.Config.CommandTimeCache["compile-something.sh 9"] = time.Duration(9 * time.Second)
// 	config.Config.CommandTimeCache["compile-something.sh 10"] = time.Duration(10 * time.Second)
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
