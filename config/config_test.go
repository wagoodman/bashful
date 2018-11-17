package config

import (
	"bytes"
	"fmt"
	"github.com/alecthomas/repr"
	"github.com/spf13/afero"
	"github.com/wagoodman/bashful/utils"
	"testing"
)

func TestMinMax(t *testing.T) {

	tester := func(arr []float64, exMin, exMax float64, exError bool) {
		min, max, err := utils.MinMax(arr)
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
		testArr := utils.RemoveOneValue(arr, value)
		if !eq(testArr, exArr) {
			t.Error("Expected", repr.String(exArr), "got", repr.String(testArr))
		}
	}

	tester([]float64{1.1, 2.2, 3.3, 4.4, 5.5, 6.6, 7.7, 8.8}, 1.1, []float64{2.2, 3.3, 4.4, 5.5, 6.6, 7.7, 8.8})
	tester([]float64{1.1, 2.2, 3.3, 4.4, 5.5, 6.6, 7.7, 8.8}, 3.14159, []float64{1.1, 2.2, 3.3, 4.4, 5.5, 6.6, 7.7, 8.8})
	tester([]float64{1.1, 1.1, 1.1, 1.1, 1.1, 1.1}, 1.1, []float64{1.1, 1.1, 1.1, 1.1, 1.1})
	tester([]float64{}, 3.14159, []float64{})

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
