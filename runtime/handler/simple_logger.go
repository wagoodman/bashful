package handler

import (
	"fmt"
	"github.com/wagoodman/bashful/config"
	"github.com/wagoodman/bashful/runtime"
	"os"
)

// this was just a (successful) experiment :) needs to be reworked

type SimpleLogger struct {
	logFile *os.File
	config  *config.Config
}

func NewSimpleLogger(config *config.Config) *SimpleLogger {
	// todo: drive this off of the config
	// todo: add logging types?
	f, err := os.OpenFile("./tmp/bashful.log", os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		panic(err)
	}
	return &SimpleLogger{
		logFile: f,
		config:  config,
	}
}

func (handler *SimpleLogger) AddRuntimeData(data *runtime.TaskStatistics) {

}

func (handler *SimpleLogger) Register(task *runtime.Task) {

}

func (handler *SimpleLogger) Unregister(task *runtime.Task) {

}

func (handler *SimpleLogger) OnEvent(task *runtime.Task, e runtime.TaskEvent) {
	// defer handler.logFile.Sync()
	if _, err := handler.logFile.WriteString(fmt.Sprintf("%+v\n", e)); err != nil {
		panic(err)
	}
}

func (handler *SimpleLogger) Close() {
	handler.logFile.Close()
}
