package runtime

import (
	"os"
	"fmt"
)

// this was just a (successful) experiment :) needs to be reworked

type LogHandler struct {
	logFile *os.File
}

func NewLogHandler() *LogHandler {
	f, err := os.OpenFile("./tmp/bashful.log", os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		panic(err)
	}
	return &LogHandler{
		logFile: f,
	}
}

func (handler *LogHandler) register(task *Task) {

}

func (handler *LogHandler) onEvent(task *Task, e event) {
	// defer handler.logFile.Sync()
	if _, err := handler.logFile.WriteString(fmt.Sprintf("%+v\n", e)); err != nil {
		panic(err)
	}
}
