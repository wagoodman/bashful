package cmd

//
// import (
// 	"github.com/wagoodman/bashful/runtime"
// 	"strconv"
// 	"testing"
// )
//
// func TestTaskErrorPolicy(t *testing.T) {
// 	var simpleYamlStr string
// 	var failedTasks []*runtime.Task
//
// 	simpleYamlStr = `
// tasks:
//   - cmd: false
//     ignore-failure: true
// `
// 	failedTasks = Execute([]byte(simpleYamlStr), map[string]string{})
// 	if len(failedTasks) > 0 {
// 		t.Error("TestTaskErrorPolicy: ignore-failure: Expected no tasks to fail, got " + strconv.Itoa(len(failedTasks)))
// 	}
//
// 	simpleYamlStr = `
// tasks:
//   - cmd: false
// `
// 	failedTasks = Execute([]byte(simpleYamlStr), map[string]string{})
// 	if len(failedTasks) != 1 {
// 		t.Error("TestTaskErrorPolicy: ack failure: Expected exactly 1 task to fail, got " + strconv.Itoa(len(failedTasks)))
// 	}
//
// 	simpleYamlStr = `
// config:
//   stop-on-failure: true
// tasks:
//   - cmd: false
//   - cmd: false
// `
// 	failedTasks = Execute([]byte(simpleYamlStr), map[string]string{})
// 	if len(failedTasks) != 1 {
// 		t.Error("TestTaskErrorPolicy: stop on failure: Expected exactly 1 task to fail, got " + strconv.Itoa(len(failedTasks)))
// 	}
//
// 	simpleYamlStr = `
// config:
//   stop-on-failure: false
// tasks:
//   - cmd: false
//   - cmd: false
// `
// 	failedTasks = Execute([]byte(simpleYamlStr), map[string]string{})
// 	if len(failedTasks) != 2 {
// 		t.Error("TestTaskErrorPolicy: do not stop on failure: Expected exactly 2 task to fail, got " + strconv.Itoa(len(failedTasks)))
// 	}
//
// }
