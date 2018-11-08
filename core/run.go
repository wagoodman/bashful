package core

import (
	"time"
	"strings"
	"fmt"
	"bytes"
	"strconv"
	"math/rand"
	"os/exec"
	"github.com/howeyc/gopass"
	color "github.com/mgutz/ansi"
	"github.com/wagoodman/bashful/task"
	"github.com/wagoodman/bashful/utils"
	"github.com/wagoodman/bashful/config"
	"github.com/wagoodman/bashful/log"
)

const (
	majorFormat = "cyan+b"
	infoFormat  = "blue+b"
	errorFormat = "red+b"
)


var (
	purple             = color.ColorFunc("magenta+h")
	red                = color.ColorFunc("red+h")
	blue               = color.ColorFunc("blue+h")
	bold               = color.ColorFunc("default+b")
)

func Run(yamlString []byte, environment map[string]string) []*task.Task {
	var err error

	config.ParseConfig(yamlString)

	if config.Config.Options.LogPath != "" {
		log.SetupLogging()
	}

	task.Open()
	storeSudoPasswd()

	DownloadAssets(task.AllTasks)

	rand.Seed(time.Now().UnixNano())


	var failedTasks []*task.Task

	tagInfo := ""
	if len(config.Config.Cli.RunTags) > 0 {
		if config.Config.Cli.ExecuteOnlyMatchedTags {
			tagInfo = " only matching tags: "
		} else {
			tagInfo = " non-tagged and matching tags: "
		}
		tagInfo += strings.Join(config.Config.Cli.RunTags, ", ")
	}

	fmt.Println(bold("Running " + tagInfo))
	log.LogToMain("Running "+tagInfo, majorFormat)

	for _, t := range task.AllTasks {
		t.Run(environment)
		failedTasks = append(failedTasks, t.FailedTasks...)

		if task.ExitSignaled {
			break
		}
	}
	log.LogToMain("Complete", majorFormat)

	err = config.Save(config.Config.EtaCachePath, &config.Config.CommandTimeCache)
	utils.CheckError(err, "Unable to save command eta cache.")

	task.Close(failedTasks)

	if len(failedTasks) > 0 {
		var buffer bytes.Buffer
		buffer.WriteString(red(" ...Some tasks failed, see below for details.\n"))

		for _, task := range failedTasks {

			buffer.WriteString("\n")
			buffer.WriteString(bold(red("• Failed task: ")) + bold(task.Config.Name) + "\n")
			buffer.WriteString(red("  ├─ command: ") + task.Config.CmdString + "\n")
			buffer.WriteString(red("  ├─ return code: ") + strconv.Itoa(task.Command.ReturnCode) + "\n")
			buffer.WriteString(red("  └─ stderr: ") + task.ErrorBuffer.String() + "\n")

		}
		log.LogToMain(buffer.String(), "")

		// we may not show the error report, but we always log it.
		if config.Config.Options.ShowFailureReport {
			fmt.Print(buffer.String())
		}

	}

	return failedTasks
}

func storeSudoPasswd() {
	var sout bytes.Buffer

	// check if there is a task that requires sudo
	requireSudo := false
	for _, t := range task.AllTasks {
		if t.Config.Sudo {
			requireSudo = true
			break
		}
		for _, subTask := range t.Children {
			if subTask.Config.Sudo {
				requireSudo = true
				break
			}
		}
	}

	if !requireSudo {
		return
	}

	// test if a password is even required for sudo
	cmd := exec.Command("/bin/sh", "-c", "sudo -Sn /bin/true")
	cmd.Stderr = &sout
	err := cmd.Run()
	requiresPassword := sout.String() == "sudo: a password is required\n"

	if requiresPassword {
		fmt.Print("[bashful] sudo password required: ")
		SudoPassword, err := gopass.GetPasswd()
		utils.CheckError(err, "Could get sudo password from user.")

		// test the given password
		cmdTest := exec.Command("/bin/sh", "-c", "sudo -S /bin/true")
		cmdTest.Stdin = strings.NewReader(string(SudoPassword) + "\n")
		err = cmdTest.Run()
		if err != nil {
			utils.ExitWithErrorMessage("Given sudo password did not work.")
		}
	} else {
		utils.CheckError(err, "Could not determine sudo access for user.")
	}
}


