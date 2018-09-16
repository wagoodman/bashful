package core

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/alecthomas/repr"
	"github.com/spf13/afero"
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

	Config.Cli.Args = []string{"First", "Second"}
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
	Config.commandTimeCache = make(map[string]time.Duration)
	Config.commandTimeCache["compile-something.sh 2"] = time.Duration(2 * time.Second)
	Config.commandTimeCache["compile-something.sh 4"] = time.Duration(4 * time.Second)
	Config.commandTimeCache["compile-something.sh 6"] = time.Duration(6 * time.Second)
	Config.commandTimeCache["compile-something.sh 9"] = time.Duration(9 * time.Second)
	Config.commandTimeCache["compile-something.sh 10"] = time.Duration(10 * time.Second)

	// load test Config yaml
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

func TestYamlInclude(t *testing.T) {
	var expStr, actStr []byte
	expStr = []byte(`
config:
    # Supress the error summary that follows
    show-failure-report: false
    show-summary-errors: true

    # Lets run more than the default 4 tasks at a time (for parallel blocks)
    max-parallel-commands: 6

    # Show an eta for each task on the screen (being shown on every line
    # with a command running)
    show-task-times: true

    # Change the color of each task status in the vertical progress bar
    success-status-color: 76
    running-status-color: 190
    pending-status-color: 237
    error-status-color: 160

x-reference-data:
  all-apps: &app-names
    - some-lib-4
    - utilities-lib
    - important-lib
    - some-app1
    - some-app3
    - some-awesome-app-5
    - watcher-app
    - yup-another-app-7
    - some-app22
    - some-awesome-app-33
    - watcher-app-33
    - yup-another-app-777

tasks:

  - name: Cloning Repos
    parallel-tasks:
      - name: "Cloning <replace>"
        cmd: example/scripts/random-worker.sh 2 <replace>
        ignore-failure: true
        for-each: *app-names
  - name: Installing dependencies
    parallel-tasks:
      - name: Installing Oracle client
        cmd: example/scripts/random-worker.sh 3
      - name: Installing Google chrome
        cmd: example/scripts/random-worker.sh 1
      - name: Installing MD helper
        cmd: example/scripts/random-worker.sh 1
      - name: Installing Bridgy
        cmd: example/scripts/random-worker.sh 1

  - name: Building Images
    cmd: example/scripts/random-worker.sh 2

  - name: Gathering Secrets
    cmd: example/scripts/random-worker.sh 2

  - name: Building and Migrating
    parallel-tasks:
      - name: "Building <replace>"
        cmd: example/scripts/random-worker.sh 2 <replace>
        ignore-failure: true
        for-each: *app-names`)

	appFs = afero.NewMemMapFs()
	appFs.MkdirAll("example", 0644)
	afero.WriteFile(appFs, "example/15-yaml-include.yml", []byte(`$include: example/common-Config.yml

x-reference-data:
  all-apps: &app-names
    - $include example/common-apps.yml

tasks:

  - name: Cloning Repos
    parallel-tasks:
      - name: "Cloning <replace>"
        cmd: example/scripts/random-worker.sh 2 <replace>
        ignore-failure: true
        for-each: *app-names

  - $include example/common-tasks.yml

  - name: Building and Migrating
    parallel-tasks:
      - name: "Building <replace>"
        cmd: example/scripts/random-worker.sh 2 <replace>
        ignore-failure: true
        for-each: *app-names`), 0644)
	afero.WriteFile(appFs, "example/common-apps.yml", []byte(`- some-lib-4
- utilities-lib
- important-lib
- some-app1
- some-app3
- some-awesome-app-5
- watcher-app
- yup-another-app-7
- some-app22
- some-awesome-app-33
- watcher-app-33
- yup-another-app-777`), 0644)
	afero.WriteFile(appFs, "example/common-tasks.yml", []byte(`- name: Installing dependencies
  parallel-tasks:
    - name: Installing Oracle client
      cmd: example/scripts/random-worker.sh 3
    - name: Installing Google chrome
      cmd: example/scripts/random-worker.sh 1
    - name: Installing MD helper
      cmd: example/scripts/random-worker.sh 1
    - name: Installing Bridgy
      cmd: example/scripts/random-worker.sh 1

- name: Building Images
  cmd: example/scripts/random-worker.sh 2

- name: Gathering Secrets
  cmd: example/scripts/random-worker.sh 2`), 0644)
	afero.WriteFile(appFs, "example/common-Config.yml", []byte(`config:
    # Supress the error summary that follows
    show-failure-report: false
    show-summary-errors: true

    # Lets run more than the default 4 tasks at a time (for parallel blocks)
    max-parallel-commands: 6

    # Show an eta for each task on the screen (being shown on every line
    # with a command running)
    show-task-times: true

    # Change the color of each task status in the vertical progress bar
    success-status-color: 76
    running-status-color: 190
    pending-status-color: 237
    error-status-color: 160`), 0644)

	contents, err := afero.ReadFile(appFs, "example/15-yaml-include.yml")
	fmt.Println(string(contents))
	// dir, err := os.Getwd()
	if err != nil {
		t.Error("Got error during assembleIncludes readfile ", err)
	} else {
		actStr = assembleIncludes(contents)
		if bytes.Compare(actStr, expStr) != 0 {
			t.Error("Expected:\n>>>", string(expStr), "<<< Got:\n>>>", string(actStr), "<<<")
		}
	}
}
