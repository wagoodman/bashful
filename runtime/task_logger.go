package runtime

import (
	"github.com/wagoodman/bashful/log"
	"github.com/wagoodman/bashful/config"
	"io/ioutil"
	"sync"
	"github.com/google/uuid"
	"os"
	"github.com/wagoodman/bashful/utils"
	"strconv"
)

type bufferedLog struct {
	// todo: do we need both logfile and logchan in a task?
	// LogChan is a channel with event log items written to the temporary logfile
	LogChan chan log.LogItem

	// LogFile is the temporary log file where all formatted stdout/stderr events are recorded
	LogFile *os.File
}

type TaskLogger struct {
	lock    sync.Mutex
	logs   map[uuid.UUID]*bufferedLog
}

func NewTaskLogger() *TaskLogger {
	if config.Config.Options.LogPath != "" {
		log.SetupLogging()
	}

	return &TaskLogger{
		logs: make(map[uuid.UUID]*bufferedLog, 0),
	}
}

func (handler *TaskLogger) doRegister(task *Task) {
	tempFile, _ := ioutil.TempFile(config.Config.LogCachePath, "")

	handler.logs[task.Id] = &bufferedLog{
		LogFile: tempFile,
		LogChan: make(chan log.LogItem),
	}
	log.LogToMain("Started Task: "+task.Config.Name, log.StyleInfo)
	go log.SingleLogger(handler.logs[task.Id].LogChan, task.Config.Name, tempFile.Name())
}

func (handler *TaskLogger) Register(task *Task) {
	if _, ok := handler.logs[task.Id]; ok {
		// ignore tasks that have already been registered
		return
	}
	handler.lock.Lock()
	defer handler.lock.Unlock()

	handler.doRegister(task)
}

func (handler *TaskLogger) Unregister(task *Task) {
	if _, ok := handler.logs[task.Id]; !ok {
		// ignore tasks that have already been unregistered
		return
	}
	handler.lock.Lock()
	defer handler.lock.Unlock()

	close(handler.logs[task.Id].LogChan)
	delete(handler.logs, task.Id)
	log.LogToMain("Completed Task: "+task.Config.Name+" (rc:"+strconv.Itoa(task.Command.ReturnCode)+")", log.StyleInfo)
}

func (handler *TaskLogger) OnEvent(task *Task, e event) {
	handler.lock.Lock()
	defer handler.lock.Unlock()

	if _, ok := handler.logs[e.Task.Id]; !ok {
		handler.doRegister(e.Task)
	}

	logInfo := handler.logs[e.Task.Id]
	if len(e.Stderr) > 0 {
		logInfo.LogChan <- log.LogItem{Name: e.Task.Config.Name, Message: utils.Red(e.Stderr) + "\n"}
	} else {
		logInfo.LogChan <- log.LogItem{Name: e.Task.Config.Name, Message: e.Stdout + "\n"}
	}

}
