package runtime

import (
	"fmt"
	"github.com/lunixbochs/vtclean"
	"github.com/wagoodman/bashful/pkg/config"
	"testing"
)

// Test harness...

const (
	actionOnEvent    = "OnEvent"
	actionRegister   = "Register"
	actionUnregister = "Unregister"
	actionClose      = "Close"
)

type testHandler struct {
	t      *testing.T
	events []*auditTestEvent
}

type auditTestEvent struct {
	action string
	task   *Task
	event  *TaskEvent
}

func newTestHander(t *testing.T) *testHandler {
	return &testHandler{
		t:      t,
		events: make([]*auditTestEvent, 0),
	}
}

func (handler *testHandler) record(e *auditTestEvent) {
	taskStr := "---"
	if e.task != nil {
		taskStr = fmt.Sprintf("%+v", e.task.Config.Name)
	}
	if e.event != nil {
		handler.t.Logf("  event %d > parent:'%+v' | task:'%s' | action:'%s' | event:%+v", len(handler.events), taskStr, e.action, e.event.Task.Config.Name, e.event)
	} else {
		handler.t.Logf("  event %d > parent:'%+v' | task:'%s' | action:'---' | event:%+v", len(handler.events), taskStr, e.action, e)
	}

	handler.events = append(handler.events, e)
}

func (handler *testHandler) AddRuntimeData(data *TaskStatistics) {

}

func (handler *testHandler) Register(task *Task) {
	handler.record(&auditTestEvent{
		action: actionRegister,
		task:   task,
	})
}

func (handler *testHandler) Unregister(task *Task) {
	handler.record(&auditTestEvent{
		action: actionUnregister,
		task:   task,
	})
}

func (handler *testHandler) OnEvent(task *Task, e TaskEvent) {
	handler.record(&auditTestEvent{
		action: actionOnEvent,
		task:   task,
		event:  &e,
	})
}

func (handler *testHandler) Close() {
	handler.record(&auditTestEvent{
		action: actionClose,
	})
}

type executorTestCase struct {
	runYaml        []byte
	expectedEvents []expectedActionEvent
	expectedEnv    map[string]string
}

func runExecutorCase(t *testing.T, testCase *executorTestCase) {
	exitSignaled = false
	handler := newTestHander(t)
	cfg, err := config.NewConfig(testCase.runYaml, nil)
	if err != nil {
		t.Fatalf("config creation failed: %v", err)
	}
	executor := newExecutor(cfg)
	executor.addEventHandler(handler)
	executor.Environment = map[string]string{
		"INITIAL_TEST_DATA": "ALSO42",
	}

	t.Logf("configured tasks: %d", len(executor.Tasks))

	t.Logf("running...")
	executor.run()

	t.Logf("validating...")

	if len(handler.events) != len(testCase.expectedEvents) {
		t.Fatalf("expected '%v' events, got %v", len(testCase.expectedEvents), len(handler.events))
	}

	for idx, expEvent := range testCase.expectedEvents {
		actualEvent := handler.events[idx]

		if expEvent.action != actualEvent.action {
			t.Fatalf("  event %d: expected action='%v', got '%v'", idx, expEvent.action, actualEvent.action)
		}

		if expEvent.taskName != "" {
			if actualEvent.task == nil {
				t.Fatalf("  event %d: expected taskName='%v', got NO TASK", idx, expEvent.taskName)
			}
			if expEvent.taskName != actualEvent.task.Config.Name {
				t.Fatalf("  event %d: expected taskName='%v', got '%v'", idx, expEvent.taskName, actualEvent.task.Config.Name)
			}

		}

		if expEvent.eventTaskName != "" {
			if actualEvent.event == nil {
				t.Fatalf("  event %d: expected eventTaskName='%v', got NO EVENT", idx, expEvent.eventTaskName)
			}
			if actualEvent.event.Task == nil {
				t.Fatalf("  event %d: expected eventTaskName='%v', got NO TASK", idx, expEvent.eventTaskName)
			}
			if expEvent.eventTaskName != actualEvent.event.Task.Config.Name {
				t.Fatalf("  event %d: expected eventTaskName='%v', got '%v'", idx, expEvent.eventTaskName, actualEvent.event.Task.Config.Name)
			}

		}

		if expEvent.action != actionOnEvent {
			continue
		}

		expEventObj := expEvent.event
		actualEventObj := actualEvent.event

		if expEventObj.Status != actualEventObj.Status {
			t.Errorf("   event %d: expected status=%v, got '%v'", idx, expEventObj.Status, actualEventObj.Status)
		}
		if expEventObj.Complete != actualEventObj.Complete {
			t.Errorf("   event %d: expected complete=%v, got '%v'", idx, expEventObj.Complete, actualEventObj.Complete)
		}
		if expEventObj.ReturnCode != actualEventObj.ReturnCode {
			t.Errorf("   event %d: expected rc=%v, got '%v'", idx, expEventObj.ReturnCode, actualEventObj.ReturnCode)
		}
		if expEventObj.Stderr != vtclean.Clean(actualEventObj.Stderr, false) {
			t.Errorf("   event %d: expected stderr='%v', got '%v'", idx, expEventObj.Stderr, actualEventObj.Stderr)
		}
		if expEventObj.Stdout != vtclean.Clean(actualEventObj.Stdout, false) {
			t.Errorf("   event %d: expected stdout='%v', got '%v'", idx, expEventObj.Stdout, actualEventObj.Stdout)
		}

	}

	for key, expValue := range testCase.expectedEnv {

		if actualValue, exists := executor.Environment[key]; exists {
			if expValue != actualValue {
				t.Errorf("   expected env[%s]=%v, got %v", key, expValue, actualValue)
			}
		} else {
			t.Errorf("   missing environment key '%s'", key)
		}

	}
}

type expectedActionEvent struct {
	action        string
	taskName      string
	eventTaskName string
	event         *TaskEvent
}

// Start testing...

func Test_Executor_run_singleTask_success(t *testing.T) {
	var runYaml = []byte(`
config:
  stop-on-failure: false
tasks:
  - name: easy task 1
    cmd: echo task1
`)
	var testCase = executorTestCase{
		runYaml: runYaml,
		expectedEvents: []expectedActionEvent{
			{action: actionRegister, taskName: "easy task 1", eventTaskName: "", event: nil},
			{action: actionOnEvent, taskName: "easy task 1", eventTaskName: "", event: &TaskEvent{nil, StatusRunning, "", "", false, -1}},
			{action: actionOnEvent, taskName: "easy task 1", eventTaskName: "", event: &TaskEvent{nil, StatusRunning, "task1", "", false, -1}},
			{action: actionOnEvent, taskName: "easy task 1", eventTaskName: "", event: &TaskEvent{nil, StatusSuccess, "", "", true, 0}},
			{action: actionUnregister, taskName: "easy task 1", eventTaskName: "", event: nil},
			{action: actionClose, taskName: "", eventTaskName: "", event: nil},
		},
		expectedEnv: map[string]string{},
	}

	runExecutorCase(t, &testCase)
}

func Test_Executor_run_singleTask_fail(t *testing.T) {
	var runYaml = []byte(`
config:
  stop-on-failure: false
tasks:
  - name: easy task 1
    cmd: false
`)
	var testCase = executorTestCase{
		runYaml: runYaml,
		expectedEvents: []expectedActionEvent{
			{action: actionRegister, taskName: "easy task 1", eventTaskName: "", event: nil},
			{action: actionOnEvent, taskName: "easy task 1", eventTaskName: "", event: &TaskEvent{nil, StatusRunning, "", "", false, -1}},
			{action: actionOnEvent, taskName: "easy task 1", eventTaskName: "", event: &TaskEvent{nil, StatusError, "", "", true, 1}},
			{action: actionUnregister, taskName: "easy task 1", eventTaskName: "", event: nil},
			{action: actionClose, taskName: "", eventTaskName: "", event: nil},
		},
		expectedEnv: map[string]string{},
	}

	runExecutorCase(t, &testCase)
}

func Test_Executor_run_serialTasks_success(t *testing.T) {
	var runYaml = []byte(`
config:
  stop-on-failure: false
tasks:
  - name: easy task 1
    cmd: true
  - name: easy task 2
    cmd: true
`)
	var testCase = executorTestCase{
		runYaml: runYaml,
		expectedEvents: []expectedActionEvent{
			{action: actionRegister, taskName: "easy task 1", eventTaskName: "", event: nil},
			{action: actionOnEvent, taskName: "easy task 1", eventTaskName: "", event: &TaskEvent{nil, StatusRunning, "", "", false, -1}},
			{action: actionOnEvent, taskName: "easy task 1", eventTaskName: "", event: &TaskEvent{nil, StatusSuccess, "", "", true, 0}},
			{action: actionUnregister, taskName: "easy task 1", eventTaskName: "", event: nil},
			{action: actionRegister, taskName: "easy task 2", eventTaskName: "", event: nil},
			{action: actionOnEvent, taskName: "easy task 2", eventTaskName: "", event: &TaskEvent{nil, StatusRunning, "", "", false, -1}},
			{action: actionOnEvent, taskName: "easy task 2", eventTaskName: "", event: &TaskEvent{nil, StatusSuccess, "", "", true, 0}},
			{action: actionUnregister, taskName: "easy task 2", eventTaskName: "", event: nil},
			{action: actionClose, taskName: "", eventTaskName: "", event: nil},
		},
		expectedEnv: map[string]string{},
	}

	runExecutorCase(t, &testCase)
}

func Test_Executor_run_serialTasks_failure_continue(t *testing.T) {
	var runYaml = []byte(`
tasks:
  - name: easy task 1
    cmd: false
    stop-on-failure: false
  - name: easy task 2
    cmd: true
`)
	var testCase = executorTestCase{
		runYaml: runYaml,
		expectedEvents: []expectedActionEvent{
			{action: actionRegister, taskName: "easy task 1", eventTaskName: "", event: nil},
			{action: actionOnEvent, taskName: "easy task 1", eventTaskName: "", event: &TaskEvent{nil, StatusRunning, "", "", false, -1}},
			{action: actionOnEvent, taskName: "easy task 1", eventTaskName: "", event: &TaskEvent{nil, StatusError, "", "", true, 1}},
			{action: actionUnregister, taskName: "easy task 1", eventTaskName: "", event: nil},
			{action: actionRegister, taskName: "easy task 2", eventTaskName: "", event: nil},
			{action: actionOnEvent, taskName: "easy task 2", eventTaskName: "", event: &TaskEvent{nil, StatusRunning, "", "", false, -1}},
			{action: actionOnEvent, taskName: "easy task 2", eventTaskName: "", event: &TaskEvent{nil, StatusSuccess, "", "", true, 0}},
			{action: actionUnregister, taskName: "easy task 2", eventTaskName: "", event: nil},
			{action: actionClose, taskName: "", eventTaskName: "", event: nil},
		},
		expectedEnv: map[string]string{},
	}

	runExecutorCase(t, &testCase)
}

func Test_Executor_run_serialTasks_failure_stop(t *testing.T) {
	var runYaml = []byte(`
config:
  stop-on-failure: true
tasks:
  - name: easy task 1
    cmd: false
  - name: easy task 2
    cmd: true
`)
	var testCase = executorTestCase{
		runYaml: runYaml,
		expectedEvents: []expectedActionEvent{
			{action: actionRegister, taskName: "easy task 1", eventTaskName: "", event: nil},
			{action: actionOnEvent, taskName: "easy task 1", eventTaskName: "", event: &TaskEvent{nil, StatusRunning, "", "", false, -1}},
			{action: actionOnEvent, taskName: "easy task 1", eventTaskName: "", event: &TaskEvent{nil, StatusError, "", "", true, 1}},
			{action: actionUnregister, taskName: "easy task 1", eventTaskName: "", event: nil},
			{action: actionClose, taskName: "", eventTaskName: "", event: nil},
		},
		expectedEnv: map[string]string{},
	}

	runExecutorCase(t, &testCase)
}

func Test_Executor_run_serialTasks_envPersist(t *testing.T) {
	var runYaml = []byte(`
config:
  stop-on-failure: false
tasks:
  - name: easy task 1
    cmd: export ANSWER=42
  - name: easy task 2
    cmd: echo $ANSWER
`)
	var testCase = executorTestCase{
		runYaml: runYaml,
		expectedEvents: []expectedActionEvent{
			{action: actionRegister, taskName: "easy task 1", eventTaskName: "", event: nil},
			{action: actionOnEvent, taskName: "easy task 1", eventTaskName: "", event: &TaskEvent{nil, StatusRunning, "", "", false, -1}},
			{action: actionOnEvent, taskName: "easy task 1", eventTaskName: "", event: &TaskEvent{nil, StatusSuccess, "", "", true, 0}},
			{action: actionUnregister, taskName: "easy task 1", eventTaskName: "", event: nil},
			{action: actionRegister, taskName: "easy task 2", eventTaskName: "", event: nil},
			{action: actionOnEvent, taskName: "easy task 2", eventTaskName: "", event: &TaskEvent{nil, StatusRunning, "", "", false, -1}},
			{action: actionOnEvent, taskName: "easy task 2", eventTaskName: "", event: &TaskEvent{nil, StatusRunning, "42", "", false, -1}},
			{action: actionOnEvent, taskName: "easy task 2", eventTaskName: "", event: &TaskEvent{nil, StatusSuccess, "", "", true, 0}},
			{action: actionUnregister, taskName: "easy task 2", eventTaskName: "", event: nil},
			{action: actionClose, taskName: "", eventTaskName: "", event: nil},
		},
		expectedEnv: map[string]string{},
	}

	runExecutorCase(t, &testCase)
}

// todo: missing parallel test cases
