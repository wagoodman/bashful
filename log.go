package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
)

type LogItem struct {
	Name    string
	Message string
}

type LogConcat struct {
	File string
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

func setupLogging() {

	err := os.MkdirAll(CachePath, 0755)
	if err != nil {
		fmt.Println("Unable to create cache dir!")
		fmt.Println(err)
		os.Exit(1)
	}
	err = os.MkdirAll(LogCachePath, 0755)
	if err != nil {
		fmt.Println("Unable to create log dir!")
		fmt.Println(err)
		os.Exit(1)
	}

	removeDirContents(LogCachePath)
	go MainLogger(Options.LogPath)
}

func SingleLogger(SingleLogChan chan LogItem, name, logPath string) {

	file, err := os.OpenFile(logPath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		fmt.Println("Unable to create log!")
		fmt.Println(err)
		os.Exit(1)
	}
	defer file.Close()
	defer func() {
		MainLogConcatChan <- LogConcat{logPath}
	}()

	logger := log.New(file, "", 0)
	logger.Println(bold("Started: " + name))

	for {
		select {
		case logObj, ok := <-SingleLogChan:
			if ok {
				logger.Print(logObj.Message)
			} else {
				SingleLogChan = nil
			}
		}
		if SingleLogChan == nil {
			break
		}
	}

	logger.Println(bold("Finished: " + name))
}

func MainLogger(logPath string) {

	file, err := os.OpenFile(logPath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		fmt.Println("Unable to create main log!")
		fmt.Println(err)
		os.Exit(1)
	}
	defer file.Close()

	logger := log.New(file, "", log.Ldate|log.Ltime)
	logger.Println(bold("Started!"))

	for {
		select {
		case logObj, ok := <-MainLogChan:
			if ok {
				logger.Print(logObj.Message)
			} else {
				MainLogChan = nil
			}

		case logCmd, ok := <-MainLogConcatChan:
			if ok {
				file.Close()

				out, err := exec.Command("bash", "-c", "cat "+logCmd.File+" >> "+logPath).CombinedOutput()

				if err != nil {
					fmt.Println("Unable to concat logs!")
					fmt.Printf("%s", out)
					fmt.Println(err)
					os.Exit(1)
				}

				file, err = os.OpenFile(logPath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
				if err != nil {
					fmt.Println("Unable to create main log!")
					fmt.Println(err)
					os.Exit(1)
				}
				logger = log.New(file, "", log.Ldate|log.Ltime)

				os.Remove(logCmd.File)
			} else {
				MainLogConcatChan = nil
			}
		}
		if MainLogChan == nil && MainLogConcatChan == nil {
			break
		}
	}

	logger.Println(bold("Finished!"))
}
