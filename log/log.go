package log

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"

	color "github.com/mgutz/ansi"
	"github.com/wagoodman/bashful/utils"
)

const (
	StyleMajor = "cyan+b"
	StyleInfo  = "blue+b"
	StyleError = "red+b"
)

var (
	enabled           bool
	mainLogChan       = make(chan LogItem)
	mainLogConcatChan = make(chan LogConcat)
)

// LogItem represents all fields in a log message
type LogItem struct {
	Name    string
	Message string
}

// LogConcat contains all metadata necessary to concatenate a subprocess log to the main log
type LogConcat struct {
	File string
}

func LogToMain(msg, format string) {
	if enabled {
		if format != "" {
			mainLogChan <- LogItem{Name: "[Main]", Message: color.Color(msg, format)}
		} else {
			mainLogChan <- LogItem{Name: "[Main]", Message: msg}
		}
	}
}

func removeDirContents(dir string) error {
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer d.Close()
	names, err := d.Readdirnames(-1)
	if err != nil {
		return err
	}
	for _, name := range names {
		err = os.RemoveAll(filepath.Join(dir, name))
		if err != nil {
			return err
		}
	}
	return nil
}

func SetupLogging(logPath, cachePath string) {
	if logPath != "" {
		enabled = true
	}

	if _, err := os.Stat(cachePath); os.IsNotExist(err) {
		err := os.MkdirAll(cachePath, 0755)
		if err != nil {
			utils.ExitWithErrorMessage("\nUnable to create log dir\n" + err.Error())
		}
	}

	removeDirContents(cachePath)
	go mainLogger(logPath)
}

// SingleLogger creats a separatly managed log (typically for an individual task to be later concatenated with the mainlog)
func SingleLogger(SingleLogChan chan LogItem, name, logPath string) {

	file, err := os.OpenFile(logPath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		utils.ExitWithErrorMessage("\nUnable to create log\n" + err.Error())
	}
	defer file.Close()
	defer func() {
		mainLogConcatChan <- LogConcat{logPath}
	}()

	logger := log.New(file, "", log.Ldate|log.Ltime)
	logger.Println(utils.Bold("Task full output: " + name))
	logger.SetFlags(0)

	for {
		logObj, ok := <-SingleLogChan
		if ok {
			logger.Print(logObj.Message)
		} else {
			SingleLogChan = nil
		}

		if SingleLogChan == nil {
			break
		}
	}

}

// mainLogger creates the main log configured by the `log-path` option
func mainLogger(logPath string) {

	file, err := os.OpenFile(logPath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		utils.ExitWithErrorMessage("\nUnable to create main log\n" + err.Error())
	}
	defer file.Close()

	logger := log.New(file, "", log.Ldate|log.Ltime)

	for {
		select {
		case logObj, ok := <-mainLogChan:
			if ok {
				logger.Print(logObj.Message)
			} else {
				mainLogChan = nil
			}

		case logCmd, ok := <-mainLogConcatChan:
			if ok {
				file.Close()

				out, err := exec.Command("bash", "-c", "cat "+logCmd.File+" >> "+logPath).CombinedOutput()

				if err != nil {
					fmt.Printf("%s\n", out)
					utils.ExitWithErrorMessage("\nUnable to concat logs\n" + err.Error())
				}

				file, err = os.OpenFile(logPath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
				if err != nil {
					utils.ExitWithErrorMessage("\nUnable to create main log\n" + err.Error())
				}
				logger = log.New(file, "", log.Ldate|log.Ltime)

				os.Remove(logCmd.File)
			} else {
				mainLogConcatChan = nil
			}
		}
		if mainLogChan == nil && mainLogConcatChan == nil {
			break
		}
	}

	logger.Println(utils.Bold("Finished!"))
}
