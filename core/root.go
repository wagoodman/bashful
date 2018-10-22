package core

import (
	"os"
	"os/signal"
	"syscall"
	"github.com/spf13/afero"
	"github.com/wagoodman/bashful/utils"
)

var (
	appFs  afero.Fs
)


func Setup() {
	appFs = afero.NewOsFs()
	sigChannel := make(chan os.Signal, 2)
	signal.Notify(sigChannel, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		for sig := range sigChannel {
			if sig == syscall.SIGINT {
				utils.ExitWithErrorMessage(red("Keyboard Interrupt"))
			} else if sig == syscall.SIGTERM {
				utils.Exit(0)
			} else {
				utils.ExitWithErrorMessage("Unknown Signal: " + sig.String())
			}
		}
	}()
}
