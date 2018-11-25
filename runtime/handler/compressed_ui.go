package handler

import (
	"fmt"
	"github.com/google/uuid"
	color "github.com/mgutz/ansi"
	"github.com/wagoodman/bashful/config"
	"github.com/wagoodman/bashful/runtime"
	"github.com/wagoodman/bashful/utils"
	"github.com/wagoodman/jotframe"
	"github.com/wayneashleyberry/terminal-dimensions"
	"strconv"
	"sync"
	"time"
)

type cUiData struct {
	Task *runtime.Task
}

type CompressedUI struct {
	lock        sync.Mutex
	config      *config.Config
	data        map[uuid.UUID]*cUiData
	startTime   time.Time
	runtimeData *runtime.RuntimeData
	frame       *jotframe.FixedFrame
}

func NewCompressedUI(config *config.Config) *CompressedUI {

	handler := &CompressedUI{
		data:      make(map[uuid.UUID]*cUiData, 0),
		startTime: time.Now(),
		frame:     jotframe.NewFixedFrame(1, false, false, false),
		config:    config,
	}

	return handler
}

func (handler *CompressedUI) AddRuntimeData(data *runtime.RuntimeData) {
	handler.runtimeData = data
}

func (handler *CompressedUI) Close() {
	handler.frame.Close()
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

	var durString, etaString, stepString, errorString string
	displayString := ""

	effectiveWidth := int(terminalWidth)

	fillColor := color.ColorCode(strconv.Itoa(handler.config.Options.ColorSuccess) + "+i")
	emptyColor := color.ColorCode(strconv.Itoa(handler.config.Options.ColorSuccess))
	if len(handler.runtimeData.FailedTasks) > 0 {
		fillColor = color.ColorCode(strconv.Itoa(handler.config.Options.ColorError) + "+i")
		emptyColor = color.ColorCode(strconv.Itoa(handler.config.Options.ColorError))
	}

	numFill := int(effectiveWidth) * len(handler.runtimeData.CompletedTasks) / handler.runtimeData.TotalTasks

	if handler.config.Options.ShowSummaryTimes {
		duration := time.Since(handler.startTime)
		durString = fmt.Sprintf(" Runtime[%s]", utils.ShowDuration(duration))

		totalEta := time.Duration(handler.config.TotalEtaSeconds) * time.Second
		remainingEta := time.Duration(totalEta.Seconds()-duration.Seconds()) * time.Second
		etaString = fmt.Sprintf(" ETA[%s]", utils.ShowDuration(remainingEta))
	}

	if len(handler.runtimeData.CompletedTasks) == handler.runtimeData.TotalTasks {
		etaString = ""
	}

	if handler.config.Options.ShowSummarySteps {
		stepString = fmt.Sprintf(" Tasks[%d/%d]", len(handler.runtimeData.CompletedTasks), handler.runtimeData.TotalTasks)
	}

	if handler.config.Options.ShowSummaryErrors {
		errorString = fmt.Sprintf(" Errors[%d]", len(handler.runtimeData.FailedTasks))
	}

	valueStr := stepString + errorString + durString + etaString

	displayString = fmt.Sprintf("%[1]*s", -effectiveWidth, fmt.Sprintf("%[1]*s", (effectiveWidth+len(valueStr))/2, valueStr))
	displayString = fillColor + displayString[:numFill] + color.Reset + emptyColor + displayString[numFill:] + color.Reset

	handler.frame.Lines()[0].WriteString(displayString)

}
