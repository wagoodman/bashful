package config

import (
	"bytes"
	"testing"

	"github.com/spf13/afero"
)

func TestYamlInclude(t *testing.T) {
	configAssembler := newAssembler(afero.NewMemMapFs())
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
        cmd: some-place/scripts/random-worker.sh 2 <replace>
        ignore-failure: true
        for-each: *app-names
  - name: Installing dependencies
    parallel-tasks:
      - name: Installing Oracle client
        cmd: some-place/scripts/random-worker.sh 3
      - name: Installing Google chrome
        cmd: some-place/scripts/random-worker.sh 1
      - name: Installing MD helper
        cmd: some-place/scripts/random-worker.sh 1
      - name: Installing Bridgy
        cmd: some-place/scripts/random-worker.sh 1

  - name: Building Images
    cmd: some-place/scripts/random-worker.sh 2

  - name: Gathering Secrets
    cmd: some-place/scripts/random-worker.sh 2

  - name: Building and Migrating
    parallel-tasks:
      - name: "Building <replace>"
        cmd: some-place/scripts/random-worker.sh 2 <replace>
        ignore-failure: true
        for-each: *app-names`)

	afero.WriteFile(configAssembler.filesystem, "some-place/run.yml", []byte(`$include: some-place/common-Config.yml

x-reference-data:
  all-apps: &app-names
    - $include some-place/common-apps.yml

tasks:

  - name: Cloning Repos
    parallel-tasks:
      - name: "Cloning <replace>"
        cmd: some-place/scripts/random-worker.sh 2 <replace>
        ignore-failure: true
        for-each: *app-names

  - $include some-place/common-tasks.yml

  - name: Building and Migrating
    parallel-tasks:
      - name: "Building <replace>"
        cmd: some-place/scripts/random-worker.sh 2 <replace>
        ignore-failure: true
        for-each: *app-names`), 0644)
	afero.WriteFile(configAssembler.filesystem, "some-place/common-apps.yml", []byte(`- some-lib-4
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
	afero.WriteFile(configAssembler.filesystem, "some-place/common-tasks.yml", []byte(`- name: Installing dependencies
  parallel-tasks:
    - name: Installing Oracle client
      cmd: some-place/scripts/random-worker.sh 3
    - name: Installing Google chrome
      cmd: some-place/scripts/random-worker.sh 1
    - name: Installing MD helper
      cmd: some-place/scripts/random-worker.sh 1
    - name: Installing Bridgy
      cmd: some-place/scripts/random-worker.sh 1

- name: Building Images
  cmd: some-place/scripts/random-worker.sh 2

- name: Gathering Secrets
  cmd: some-place/scripts/random-worker.sh 2`), 0644)
	afero.WriteFile(configAssembler.filesystem, "some-place/common-Config.yml", []byte(`config:
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

	contents, err := afero.ReadFile(configAssembler.filesystem, "some-place/run.yml")

	if err != nil {
		t.Error("Got error during assemble readfile ", err)
	} else {
		actStr = configAssembler.assemble(contents)
		if bytes.Compare(actStr, expStr) != 0 {
			t.Error("Expected:\n>>>", string(expStr), "<<< Got:\n>>>", string(actStr), "<<<")
		}
	}
}
