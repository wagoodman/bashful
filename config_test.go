package main

import (
	"strings"
	"testing"
	"time"

	"github.com/alecthomas/repr"
)

func TestMinMax(t *testing.T) {

	tester := func(arr []float64, exMin, exMax float64, exError bool) {
		min, max, err := MinMax(arr)
		if min != exMin {
			t.Error("Expected min=", exMin, "got", min)
		}
		if max != exMax {
			t.Error("Expected max=", exMax, "got", max)
		}
		if err != nil && !exError {
			t.Error("Expected no error, got error:", err)
		} else if err == nil && exError {
			t.Error("Expected an error but there wasn't one")
		}
	}

	tester([]float64{1.1, 2.2, 3.3, 4.4, 5.5, 6.6, 7.7, 8.8}, 1.1, 8.8, false)
	tester([]float64{1.1, 1.1, 1.1, 1.1, 1.1, 1.1}, 1.1, 1.1, false)
	tester([]float64{}, 0, 0, true)

}

func TestRemoveOneValue(t *testing.T) {
	eq := func(a, b []float64) bool {

		if a == nil && b == nil {
			return true
		}

		if a == nil || b == nil {
			return false
		}

		if len(a) != len(b) {
			return false
		}

		for i := range a {
			if a[i] != b[i] {
				return false
			}
		}

		return true
	}

	tester := func(arr []float64, value float64, exArr []float64) {
		testArr := removeOneValue(arr, value)
		if !eq(testArr, exArr) {
			t.Error("Expected", repr.String(exArr), "got", repr.String(testArr))
		}
	}

	tester([]float64{1.1, 2.2, 3.3, 4.4, 5.5, 6.6, 7.7, 8.8}, 1.1, []float64{2.2, 3.3, 4.4, 5.5, 6.6, 7.7, 8.8})
	tester([]float64{1.1, 2.2, 3.3, 4.4, 5.5, 6.6, 7.7, 8.8}, 3.14159, []float64{1.1, 2.2, 3.3, 4.4, 5.5, 6.6, 7.7, 8.8})
	tester([]float64{1.1, 1.1, 1.1, 1.1, 1.1, 1.1}, 1.1, []float64{1.1, 1.1, 1.1, 1.1, 1.1})
	tester([]float64{}, 3.14159, []float64{})

}

func TestCommandArguments(t *testing.T) {
	yamlStr := `
tasks:
  - cmd: command-with-args $1 $2
    name: Arg1=$1 Arg2=$2
  - cmd: all-args $*
    name: Args=$*
`

	config.Cli.Args = []string{"First", "Second"}
	parseRunYaml([]byte(yamlStr))
	tasks := CreateTasks()
	if len(tasks) != 2 {
		t.Error("Expected two tasks. Got: ", len(tasks))
	}

	if tasks[0].Config.CmdString != "command-with-args First Second" {
		t.Error("Expected arguments to be replaced. Got: ", tasks[0].Config.CmdString)
	}
	if tasks[0].Config.Name != "Arg1=First Arg2=Second" {
		t.Error("Expected arguments to be replaced in task name. Got: ", tasks[0].Config.Name)
	}

	if tasks[1].Config.CmdString != "all-args First Second" {
		t.Error("Expected all arguments to be replaced. Got: ", tasks[1].Config.CmdString)
	}
	if tasks[1].Config.Name != "Args=First Second" {
		t.Error("Expected all arguments to be replaced in task name. Got: ", tasks[1].Config.Name)
	}
}

func TestCreateTasks_SuccessfulParse(t *testing.T) {
	var expStr, actStr string
	var expOpt, actOpt bool
	var exNum int
	simpleYamlStr := `
tasks:
  - cmd: random-worker.sh 10
  - name: Compiling source
    parallel-tasks:
      - cmd: compile-something.sh 2
      - cmd: compile-something.sh 9
        stop-on-failure: false
        show-output: false
      - cmd: compile-something.sh 6
      - cmd: compile-something.sh 4 <replace>
        for-each:
          - plug 4
          - plug 5
          - plug 6
  - name: some random task name <replace>
    cmd: random-worker.sh 2
    for-each:
      - plug 3
  - cmd: random-worker.sh 10 <replace>
    for-each:
      - plug 1
      - plug 2
`
	// load test time cache
	config.commandTimeCache = make(map[string]time.Duration)
	config.commandTimeCache["compile-something.sh 2"] = time.Duration(2 * time.Second)
	config.commandTimeCache["compile-something.sh 4"] = time.Duration(4 * time.Second)
	config.commandTimeCache["compile-something.sh 6"] = time.Duration(6 * time.Second)
	config.commandTimeCache["compile-something.sh 9"] = time.Duration(9 * time.Second)
	config.commandTimeCache["compile-something.sh 10"] = time.Duration(10 * time.Second)

	// load test config yaml
	parseRunYaml([]byte(simpleYamlStr))
	// create and inflate tasks
	tasks := CreateTasks()

	// validate test task yaml

	exNum = 5
	if len(tasks) != exNum {
		t.Error("Expected", exNum, "tasks got", len(tasks))
	}

	exNum = 6
	if len(tasks[1].Children) != exNum {
		t.Error("Expected", exNum, "parallel tasks got", len(tasks[1].Children))
	}

	// ensure that names are set properly

	expStr, actStr = "Compiling source", tasks[1].Config.Name
	if actStr != expStr {
		t.Error("Expected name:", expStr, "got name:", actStr)
	}

	expStr, actStr = "some random task name plug 3", tasks[2].Config.Name
	if actStr != expStr {
		t.Error("Expected name:", expStr, "got name:", actStr)
	}

	// check the names of the top task list

	for _, taskIndex := range []int{0, 3, 4} {
		repr.Println(tasks[taskIndex].Config)
		expStr, actStr = tasks[taskIndex].Config.CmdString, tasks[taskIndex].Config.Name
		if actStr != expStr {
			t.Error("Expected name:", expStr, "got name:", actStr)
		}
	}

	// check the names of the top parallel list
	for taskIndex := 0; taskIndex < len(tasks[1].Children); taskIndex++ {
		expStr, actStr = tasks[1].Children[taskIndex].Config.CmdString, tasks[1].Children[taskIndex].Config.Name
		if actStr != expStr {
			t.Error("Expected name:", expStr, "got name:", actStr)
		}
	}

	// ensure that names and commands of for-each tasks fill in the ? character with params
	for _, taskIndex := range []int{3, 4} {
		expStr, actStr = tasks[taskIndex].Config.CmdString, tasks[taskIndex].Config.Name
		if actStr != expStr {
			t.Error("Expected name:", expStr, "got name:", actStr)
		}
		if !strings.Contains(tasks[taskIndex].Config.Name, "plug") {
			t.Error("Expected name to contain 'plug' but got:", actStr)
		}
		if !strings.Contains(tasks[taskIndex].Config.CmdString, "plug") {
			t.Error("Expected cmd to contain 'plug' but got:", actStr)
		}
	}

	expStr, actStr = "some random task name plug 3", tasks[2].Config.Name
	if actStr != expStr {
		t.Error("Expected name:", expStr, "got name:", actStr)
	}

	expStr, actStr = "random-worker.sh 2", tasks[2].Config.CmdString
	if actStr != expStr {
		t.Error("Expected cmd:", expStr, "got cmd:", actStr)
	}

	// ensure stop on fail and show output can be overridden to false
	expOpt, actOpt = false, tasks[1].Children[1].Config.ShowTaskOutput
	if actOpt != expOpt {
		t.Error("Expected name:", expOpt, "got name:", actOpt)
	}

	expOpt, actOpt = false, tasks[1].Children[1].Config.StopOnFailure
	if actOpt != expOpt {
		t.Error("Expected name:", expOpt, "got name:", actOpt)
	}

}
