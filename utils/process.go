// +build !windows

package utils

import (
	"syscall"
)

func KillProcess(pid int) {
	syscall.Kill(-pid, syscall.SIGKILL)
}

func GetSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setpgid: true}
}
