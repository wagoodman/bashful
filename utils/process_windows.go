package utils

import (
	"syscall"
)

func KillProcess(pid int) {

}

func GetSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{}
}
