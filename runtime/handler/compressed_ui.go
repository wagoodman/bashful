package handler

import (
	"github.com/wagoodman/bashful/config"
	"strconv"
	"time"
	"fmt"
	"github.com/wagoodman/bashful/utils"

	"github.com/wayneashleyberry/terminal-dimensions"
	color "github.com/mgutz/ansi"
	"github.com/google/uuid"
	"sync"
	"github.com/wagoodman/bashful/runtime"

)

type cUiData struct {
	Task *runtime.Task
}

type CompressedUI struct {
	lock            sync.Mutex
	data            map[uuid.UUID]*cUiData
	startTime       time.Time
	executor        *runtime.Executor
}

func NewCompressedUI() *CompressedUI {

	handler := &CompressedUI{
		data:            make(map[uuid.UUID]*cUiData, 0),
		startTime:       time.Now(),
	}

	return handler
}

// todo: move footer logic based on jotframe requirements
func (handler *CompressedUI) Close() {

}

func (handler *CompressedUI) Unregister(task *runtime.Task) {
	if _, ok := handler.data[task.Id]; !ok {
		// ignore data that have already been unregistered
		return
	}
	handler.lock.Lock()
	defer handler.lock.Unlock()
	delete(handler.data, task.Id)
}

func (handler *CompressedUI) doRegister(task *runtime.Task) {
	if _, ok := handler.data[task.Id]; ok {
		// ignore data that have already been registered
		return
	}

	// todo: this is hackey
	if handler.executor == nil && task.Executor != nil {
		handler.executor = task.Executor
	}

	handler.data[task.Id] = &cUiData{
		Task: task,
	}
	for _, subTask := range task.Children {
		handler.data[subTask.Id] = &cUiData{
			Task: subTask,
		}
	}
}

func (handler *CompressedUI) Register(task *runtime.Task) {
	handler.lock.Lock()
	defer handler.lock.Unlock()

	handler.doRegister(task)

}

func (handler *CompressedUI) OnEvent(task *runtime.Task, e runtime.TaskEvent) {
	handler.lock.Lock()
	defer handler.lock.Unlock()

	handler.displayTask(e.Task)
}

func (handler *CompressedUI) displayTask(task *runtime.Task) {

	// todo: error handling
	if _, ok := handler.data[task.Id]; !ok {
		return
	}

	terminalWidth, _ := terminaldimensions.Width()
	theScreen := GetScreen()

	var durString, etaString, stepString, errorString string
	displayString := ""

	effectiveWidth := int(terminalWidth)

	fillColor := color.ColorCode(strconv.Itoa(config.Config.Options.ColorSuccess) + "+i")
	emptyColor := color.ColorCode(strconv.Itoa(config.Config.Options.ColorSuccess))
	if len(task.Executor.FailedTasks) > 0 {
		fillColor = color.ColorCode(strconv.Itoa(config.Config.Options.ColorError) + "+i")
		emptyColor = color.ColorCode(strconv.Itoa(config.Config.Options.ColorError))
	}

	numFill := int(effectiveWidth) * len(task.Executor.CompletedTasks) / task.Executor.TotalTasks

	if config.Config.Options.ShowSummaryTimes {
		duration := time.Since(handler.startTime)
		durString = fmt.Sprintf(" Runtime[%s]", utils.ShowDuration(duration))

		totalEta := time.Duration(config.Config.TotalEtaSeconds) * time.Second
		remainingEta := time.Duration(totalEta.Seconds()-duration.Seconds()) * time.Second
		etaString = fmt.Sprintf(" ETA[%s]", utils.ShowDuration(remainingEta))
	}

	if len(task.Executor.CompletedTasks) == task.Executor.TotalTasks {
		etaString = ""
	}

	if config.Config.Options.ShowSummarySteps {
		stepString = fmt.Sprintf(" Tasks[%d/%d]", len(task.Executor.CompletedTasks), task.Executor.TotalTasks)
	}

	if config.Config.Options.ShowSummaryErrors {
		errorString = fmt.Sprintf(" Errors[%d]", len(task.Executor.FailedTasks))
	}

	valueStr := stepString + errorString + durString + etaString

	displayString = fmt.Sprintf("%[1]*s", -effectiveWidth, fmt.Sprintf("%[1]*s", (effectiveWidth+len(valueStr))/2, valueStr))
	displayString = fillColor + displayString[:numFill] + color.Reset + emptyColor + displayString[numFill:] + color.Reset

	theScreen.Display(displayString, 0)


}
