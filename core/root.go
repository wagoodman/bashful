package core

import (
	"os"
	"os/signal"
	"syscall"
	"github.com/wagoodman/bashful/utils"
)


func Setup() {
	sigChannel := make(chan os.Signal, 2)
	signal.Notify(sigChannel, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		for sig := range sigChannel {
			if sig == syscall.SIGINT {
				utils.ExitWithErrorMessage(utils.Red("Keyboard Interrupt"))
			} else if sig == syscall.SIGTERM {
				utils.Exit(0)
			} else {
				utils.ExitWithErrorMessage("Unknown Signal: " + sig.String())
			}
		}
	}()
}
