package runtime

import (
	"bytes"
	"github.com/wagoodman/bashful/config"
	"github.com/wagoodman/bashful/utils"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"
)

func newCommand(taskConfig config.TaskConfig) command {
	shell := os.Getenv("SHELL")
	if len(shell) == 0 {
		shell = "sh"
	}

	readFd, writeFd, err := os.Pipe()
	utils.CheckError(err, "Could not open env pipe for child shell")

	sudoCmd := ""
	if taskConfig.Sudo {
		sudoCmd = "sudo -S "
	}
	cmd := exec.Command(shell, "-c", sudoCmd+taskConfig.CmdString+"; BASHFUL_RC=$?; env >&3; exit $BASHFUL_RC")
	cmd.Stdin = strings.NewReader(string(sudoPassword) + "\n")

	// Set current working directory; default is empty
	cmd.Dir = taskConfig.CwdString

	// allow the child process to provide env vars via a pipe (FD3)
	cmd.ExtraFiles = []*os.File{writeFd}

	// set this command as a process group
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	return command{
		Environment:      map[string]string{},
		ReturnCode:       -1,
		EnvReadFile:      readFd,
		Cmd:              cmd,
		EstimatedRuntime: time.Duration(-1),
		errorBuffer:      bytes.NewBufferString(""),
	}
}

func (cmd *command) addEstimatedRuntime(duration time.Duration) {
	cmd.EstimatedRuntime = duration
}
