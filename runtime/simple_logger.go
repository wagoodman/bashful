package runtime

import (
	"os"
	"fmt"
)

// this was just a (successful) experiment :) needs to be reworked

type SimpleLogger struct {
	logFile *os.File
}

func NewSimpleLogger() *SimpleLogger {
	f, err := os.OpenFile("./tmp/bashful.log", os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		panic(err)
	}
	return &SimpleLogger{
		logFile: f,
	}
}

func (handler *SimpleLogger) Register(task *Task) {

}

func (handler *SimpleLogger) Unregister(task *Task) {

}

func (handler *SimpleLogger) OnEvent(task *Task, e event) {
	// defer handler.logFile.Sync()
	if _, err := handler.logFile.WriteString(fmt.Sprintf("%+v\n", e)); err != nil {
		panic(err)
	}
}
