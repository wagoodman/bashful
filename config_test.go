package main

import (
	"strings"
	"testing"
	"time"

	yaml "gopkg.in/yaml.v2"

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
      - cmd: compile-something.sh 4 ?
        for-each:
          - plug 4
          - plug 5
          - plug 6
  - name: some random task name ?
    cmd: random-worker.sh 2
    for-each:
      - plug 3
  - cmd: random-worker.sh 10 ?
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
	config.Options = defaultOptions()
	err := yaml.Unmarshal([]byte(simpleYamlStr), &config)
	if err != nil {
		t.Error("Expected no error, got error:", err)
	}

	// create and inflate tasks
	createTasks()

	// validate test task yaml

	exNum = 5
	if len(config.Tasks) != exNum {
		t.Error("Expected", exNum, "tasks got", len(config.Tasks))
	}

	exNum = 6
	if len(config.Tasks[1].ParallelTasks) != exNum {
		t.Error("Expected", exNum, "parallel tasks got", len(config.Tasks[1].ParallelTasks))
	}

	// ensure that names are set properly

	expStr, actStr = "Compiling source", config.Tasks[1].Name
	if actStr != expStr {
		t.Error("Expected name:", expStr, "got name:", actStr)
	}

	expStr, actStr = "some random task name plug 3", config.Tasks[2].Name
	if actStr != expStr {
		t.Error("Expected name:", expStr, "got name:", actStr)
	}

	// check the names of the top task list
	for _, taskIndex := range []int{0, 3, 4} {
		expStr, actStr = config.Tasks[taskIndex].CmdString, config.Tasks[taskIndex].Name
		if actStr != expStr {
			t.Error("Expected name:", expStr, "got name:", actStr)
		}
	}

	// check the names of the top parallel list
	for taskIndex := 0; taskIndex < len(config.Tasks[1].ParallelTasks); taskIndex++ {
		expStr, actStr = config.Tasks[1].ParallelTasks[taskIndex].CmdString, config.Tasks[1].ParallelTasks[taskIndex].Name
		if actStr != expStr {
			t.Error("Expected name:", expStr, "got name:", actStr)
		}
	}

	// ensure that names and commands of for-each tasks fill in the ? character with params
	for _, taskIndex := range []int{3, 4} {
		expStr, actStr = config.Tasks[taskIndex].CmdString, config.Tasks[taskIndex].Name
		if actStr != expStr {
			t.Error("Expected name:", expStr, "got name:", actStr)
		}
		if !strings.Contains(config.Tasks[taskIndex].Name, "plug") {
			t.Error("Expected name to contain 'plug' but got:", actStr)
		}
		if !strings.Contains(config.Tasks[taskIndex].CmdString, "plug") {
			t.Error("Expected cmd to contain 'plug' but got:", actStr)
		}
	}

	expStr, actStr = "some random task name plug 3", config.Tasks[2].Name
	if actStr != expStr {
		t.Error("Expected name:", expStr, "got name:", actStr)
	}

	expStr, actStr = "random-worker.sh 2", config.Tasks[2].CmdString
	if actStr != expStr {
		t.Error("Expected cmd:", expStr, "got cmd:", actStr)
	}

	// ensure stop on fail and show output can be overridden to false
	expOpt, actOpt = false, config.Tasks[1].ParallelTasks[1].ShowTaskOutput
	if actOpt != expOpt {
		t.Error("Expected name:", expOpt, "got name:", actOpt)
	}

	expOpt, actOpt = false, config.Tasks[1].ParallelTasks[1].StopOnFailure
	if actOpt != expOpt {
		t.Error("Expected name:", expOpt, "got name:", actOpt)
	}

}
