package config

import (
	"github.com/deckarep/golang-set"
	"github.com/wagoodman/bashful/utils"
	"testing"
)

func Test_Compile_SingleTask(t *testing.T) {
	runYaml := []byte(`
tasks:
  - name: Thing-a-ma-bob
    cmd: ./do/a/thing`)

	config, err := NewConfig(runYaml, nil)
	if err != nil {
		t.Errorf("expected no config error, got %+v", err)
	}

	if len(config.TaskConfigs) != 1 {
		t.Errorf("expected 1 task, got %d", len(config.TaskConfigs))
	}

	var collection = utils.TestCollection{
		Collection: utils.InterfaceSlice(config.TaskConfigs),
		Cases: []utils.TestCase{
			{Index: 0, ExpectedValue: "Thing-a-ma-bob", ActualName: "Name"},
			{Index: 0, ExpectedValue: "./do/a/thing", ActualName: "CmdString"},
		},
	}

	utils.AssertTestCases(t, collection)

}

func Test_Compile_MultipleSerialTasks(t *testing.T) {
	runYaml := []byte(`
tasks:
  - name: Do-a-thing
    cmd: ./do/a/thing-1
  - name: Throw-a-thing
    cmd: ./do/a/thing-2
    ignore-failure: true
  - name: Do-another-thing
    cmd: ./do/a/thing-3`)

	config, err := NewConfig(runYaml, nil)
	if err != nil {
		t.Errorf("expected no config error, got %+v", err)
	}

	if len(config.TaskConfigs) != 3 {
		t.Errorf("expected 3 tasks, got %d", len(config.TaskConfigs))
	}

	var collection = utils.TestCollection{
		Collection: utils.InterfaceSlice(config.TaskConfigs),
		Cases: []utils.TestCase{
			{Index: 0, ExpectedValue: "Do-a-thing", ActualName: "Name"},
			{Index: 0, ExpectedValue: "./do/a/thing-1", ActualName: "CmdString"},
			{Index: 0, ExpectedValue: false, ActualName: "IgnoreFailure"},

			{Index: 1, ExpectedValue: "Throw-a-thing", ActualName: "Name"},
			{Index: 1, ExpectedValue: "./do/a/thing-2", ActualName: "CmdString"},
			{Index: 1, ExpectedValue: true, ActualName: "IgnoreFailure"},

			{Index: 2, ExpectedValue: "Do-another-thing", ActualName: "Name"},
			{Index: 2, ExpectedValue: "./do/a/thing-3", ActualName: "CmdString"},
			{Index: 2, ExpectedValue: false, ActualName: "IgnoreFailure"},
		},
	}
	utils.AssertTestCases(t, collection)
}

func Test_Compile_ParallelTask(t *testing.T) {
	runYaml := []byte(`
tasks:
  - name: Do-things
    parallel-tasks:
    - name: Throw-a-thing
      cmd: ./do/a/thing-2
      ignore-failure: true
    - name: Do-another-thing
      cmd: ./do/a/thing-3`)

	config, err := NewConfig(runYaml, nil)
	if err != nil {
		t.Errorf("expected no config error, got %+v", err)
	}

	actualLen := len(config.TaskConfigs)
	if actualLen != 1 {
		t.Errorf("expected 1 tasks, got %d", actualLen)
	}

	testCases := []utils.TestCase{
		{ExpectedValue: "Do-things", ActualName: "Name"},
	}

	for _, theCase := range testCases {
		utils.AssertTestCase(t, config.TaskConfigs[0], theCase)
	}

	actualLen = len(config.TaskConfigs[0].ParallelTasks)
	if actualLen != 2 {
		t.Errorf("expected 2 parallel tasks, got %d", actualLen)
	}

	var collection = utils.TestCollection{
		Collection: utils.InterfaceSlice(config.TaskConfigs[0].ParallelTasks),
		Cases: []utils.TestCase{
			{Index: 0, ExpectedValue: "Throw-a-thing", ActualName: "Name"},
			{Index: 0, ExpectedValue: "./do/a/thing-2", ActualName: "CmdString"},
			{Index: 0, ExpectedValue: true, ActualName: "IgnoreFailure"},

			{Index: 1, ExpectedValue: "Do-another-thing", ActualName: "Name"},
			{Index: 1, ExpectedValue: "./do/a/thing-3", ActualName: "CmdString"},
			{Index: 1, ExpectedValue: false, ActualName: "IgnoreFailure"},
		},
	}
	utils.AssertTestCases(t, collection)
}

func Test_Compile_TemplateLoop(t *testing.T) {
	runYaml := []byte(`
x-reference-data:
  all-apps: &app-names
    - app-1
    - app-2
    - app-3
    - app-4

tasks:
  - name: Cloning Repos
    parallel-tasks:
      - name: "Cloning <replace>"
        cmd: some-place/scripts/random-worker.sh 2 <replace>
        ignore-failure: true
        for-each: *app-names`)

	config, err := NewConfig(runYaml, nil)
	if err != nil {
		t.Errorf("expected no config error, got %+v", err)
	}

	actualLen := len(config.TaskConfigs)
	if actualLen != 1 {
		t.Errorf("expected 1 tasks, got %d", actualLen)
	}

	testCases := []utils.TestCase{
		{ExpectedValue: "Cloning Repos", ActualName: "Name"},
	}

	for _, theCase := range testCases {
		utils.AssertTestCase(t, config.TaskConfigs[0], theCase)
	}

	actualLen = len(config.TaskConfigs[0].ParallelTasks)
	if actualLen != 4 {
		t.Errorf("expected 4 parallel tasks, got %d", actualLen)
	}

	var collection = utils.TestCollection{
		Collection: utils.InterfaceSlice(config.TaskConfigs[0].ParallelTasks),
		Cases: []utils.TestCase{
			{Index: 0, ExpectedValue: "Cloning app-1", ActualName: "Name"},
			{Index: 0, ExpectedValue: "some-place/scripts/random-worker.sh 2 app-1", ActualName: "CmdString"},
			{Index: 0, ExpectedValue: true, ActualName: "IgnoreFailure"},

			{Index: 1, ExpectedValue: "Cloning app-2", ActualName: "Name"},
			{Index: 1, ExpectedValue: "some-place/scripts/random-worker.sh 2 app-2", ActualName: "CmdString"},
			{Index: 1, ExpectedValue: true, ActualName: "IgnoreFailure"},

			{Index: 2, ExpectedValue: "Cloning app-3", ActualName: "Name"},
			{Index: 2, ExpectedValue: "some-place/scripts/random-worker.sh 2 app-3", ActualName: "CmdString"},
			{Index: 2, ExpectedValue: true, ActualName: "IgnoreFailure"},

			{Index: 3, ExpectedValue: "Cloning app-4", ActualName: "Name"},
			{Index: 3, ExpectedValue: "some-place/scripts/random-worker.sh 2 app-4", ActualName: "CmdString"},
			{Index: 3, ExpectedValue: true, ActualName: "IgnoreFailure"},
		},
	}
	utils.AssertTestCases(t, collection)
}

func Test_Compile_TagInheritance(t *testing.T) {
	runYaml := []byte(`
x-reference-data:
  all-apps: &app-names
    - app-1
    - app-2
    - app-3
    - app-4

tasks:
  - tags:
      - sweet
      - awesome
    parallel-tasks:
      - name: "Cloning <replace>"
        cmd: some-place/scripts/random-worker.sh 2 <replace>
        for-each: *app-names`)

	config, err := NewConfig(runYaml, nil)
	if err != nil {
		t.Errorf("expected no config error, got %+v", err)
	}

	actualLen := len(config.TaskConfigs)
	if actualLen != 1 {
		t.Errorf("expected 1 tasks, got %d", actualLen)
	}

	// ensure the proper tags are on the parent task
	expectedTags := []string{"sweet", "awesome"}
	for idx, actualTag := range config.TaskConfigs[0].Tags {
		if actualTag != expectedTags[idx] {
			t.Errorf("expected tag='%s', got '%s'", expectedTags[idx], actualTag)
		}
		if !config.TaskConfigs[0].TagSet.Contains(expectedTags[idx]) {
			t.Errorf("expected tag='%s' to be in the TagSet but was not", expectedTags[idx])
		}
	}

	actualLen = len(config.TaskConfigs[0].Tags)
	if actualLen != 2 {
		t.Errorf("expected 2 tags, got %d", actualLen)
	}

	actualLen = len(config.TaskConfigs[0].ParallelTasks)
	if actualLen != 4 {
		t.Errorf("expected 4 parallel tasks, got %d", actualLen)
	}

	// ensure the proper tags are on the child tasks
	var collection = utils.TestCollection{
		Collection: utils.InterfaceSlice(config.TaskConfigs[0].ParallelTasks),
		Cases: []utils.TestCase{
			{Index: 0, ExpectedValue: "Cloning app-1", ActualName: "Name"},
			{Index: 1, ExpectedValue: "Cloning app-2", ActualName: "Name"},
			{Index: 2, ExpectedValue: "Cloning app-3", ActualName: "Name"},
			{Index: 3, ExpectedValue: "Cloning app-4", ActualName: "Name"},
		},
	}
	utils.AssertTestCases(t, collection)

	for _, taskConfig := range config.TaskConfigs[0].ParallelTasks {
		actualLen = len(taskConfig.Tags)
		if actualLen != 2 {
			t.Errorf("expected 2 tags, got %d", actualLen)
		}
		for idx, actualTag := range taskConfig.Tags {
			if actualTag != expectedTags[idx] {
				t.Errorf("expected tag='%s', got '%s'", expectedTags[idx], actualTag)
			}
			if !taskConfig.TagSet.Contains(expectedTags[idx]) {
				t.Errorf("expected tag='%s' to be in the TagSet but was not", expectedTags[idx])
			}
		}
	}
}

func Test_Compile_TagSelection(t *testing.T) {
	runYaml := []byte(`
x-reference-data:
  all-apps: &app-names
    - app-1
    - app-2
    - app-3
    - app-4

tasks:
  - tags:
      - kinda-sweet
      - awesome
    parallel-tasks:
      - name: "Cloning <replace>"
        cmd: some-place/scripts/random-worker.sh 2 <replace>
        for-each: *app-names

  - name: a-sweet-thing
    cmd: ./do/sweet.sh
    tags: 
      - sweet
      - very-awesome

  - name: just-a-thing
    cmd: ./do/things.sh

`)

	cli := Cli{
		RunTags:   []string{"sweet"},
		RunTagSet: mapset.NewSet(),
	}
	for _, tag := range cli.RunTags {
		cli.RunTagSet.Add(tag)
	}

	config, err := NewConfig(runYaml, &cli)
	if err != nil {
		t.Errorf("expected no config error, got %+v", err)
	}

	actualLen := len(config.TaskConfigs)
	if actualLen != 2 {
		t.Errorf("expected 2 tasks, got %d", actualLen)
	}

	var collection = utils.TestCollection{
		Collection: utils.InterfaceSlice(config.TaskConfigs),
		Cases: []utils.TestCase{
			{Index: 0, ExpectedValue: "a-sweet-thing", ActualName: "Name"},
			{Index: 0, ExpectedValue: "./do/sweet.sh", ActualName: "CmdString"},

			{Index: 1, ExpectedValue: "just-a-thing", ActualName: "Name"},
			{Index: 1, ExpectedValue: "./do/things.sh", ActualName: "CmdString"},
		},
	}
	utils.AssertTestCases(t, collection)

	// ensure the proper tags are in place
	expectedTags := []string{"sweet", "very-awesome"}
	for idx, actualTag := range config.TaskConfigs[0].Tags {
		actualLen = len(config.TaskConfigs[0].Tags)
		if actualLen != 2 {
			t.Errorf("expected 2 tags, got %d", actualLen)
		}
		if actualTag != expectedTags[idx] {
			t.Errorf("expected tag='%s', got '%s'", expectedTags[idx], actualTag)
		}
		if !config.TaskConfigs[0].TagSet.Contains(expectedTags[idx]) {
			t.Errorf("expected tag='%s' to be in the TagSet but was not", expectedTags[idx])
		}
	}

}

func Test_Compile_OnlyTagSelection(t *testing.T) {
	runYaml := []byte(`
x-reference-data:
  all-apps: &app-names
    - app-1
    - app-2
    - app-3
    - app-4

tasks:
  - tags:
      - kinda-sweet
      - awesome
    parallel-tasks:
      - name: "Cloning <replace>"
        cmd: some-place/scripts/random-worker.sh 2 <replace>
        for-each: *app-names

  - name: a-sweet-thing
    cmd: ./do/sweet.sh
    tags: 
      - sweet
      - very-awesome

  - name: just-a-thing
    cmd: ./do/things.sh

`)

	cli := Cli{
		RunTags:                []string{"sweet"},
		RunTagSet:              mapset.NewSet(),
		ExecuteOnlyMatchedTags: true,
	}
	for _, tag := range cli.RunTags {
		cli.RunTagSet.Add(tag)
	}

	config, err := NewConfig(runYaml, &cli)
	if err != nil {
		t.Errorf("expected no config error, got %+v", err)
	}

	actualLen := len(config.TaskConfigs)
	if actualLen != 1 {
		t.Errorf("expected 1 tasks, got %d", actualLen)
	}

	var collection = utils.TestCollection{
		Collection: utils.InterfaceSlice(config.TaskConfigs),
		Cases: []utils.TestCase{
			{Index: 0, ExpectedValue: "a-sweet-thing", ActualName: "Name"},
			{Index: 0, ExpectedValue: "./do/sweet.sh", ActualName: "CmdString"},
		},
	}
	utils.AssertTestCases(t, collection)

	// ensure the proper tags are in place
	expectedTags := []string{"sweet", "very-awesome"}
	for idx, actualTag := range config.TaskConfigs[0].Tags {
		actualLen = len(config.TaskConfigs[0].Tags)
		if actualLen != 2 {
			t.Errorf("expected 2 tags, got %d", actualLen)
		}
		if actualTag != expectedTags[idx] {
			t.Errorf("expected tag='%s', got '%s'", expectedTags[idx], actualTag)
		}
		if !config.TaskConfigs[0].TagSet.Contains(expectedTags[idx]) {
			t.Errorf("expected tag='%s' to be in the TagSet but was not", expectedTags[idx])
		}
	}

}
