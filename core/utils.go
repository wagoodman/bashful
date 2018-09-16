package core

import (
	"fmt"
	"runtime"
	"time"
	"os"
)

func ExitWithErrorMessage(msg string) {
	cleanup()
	fmt.Fprintln(os.Stderr, red(msg))
	os.Exit(1)
}

func Exit(rc int) {
	cleanup()
	os.Exit(rc)
}

func cleanup() {
	// stop any running tasks
	for _, task := range allTasks {
		task.Kill()
	}

	// move the cursor past the used screen realestate
	newScreen().MovePastFrame(true)

	// show the cursor again
	fmt.Print("\033[?25h") // show cursor
}

func CheckError(err error, message string) {
	if err != nil {
		fmt.Println(red("Error:"))
		_, file, line, _ := runtime.Caller(1)
		fmt.Println("Line:", line, "\tFile:", file, "\n", err)
		ExitWithErrorMessage(message)
	}
}

func doesFileExist(name string) bool {
	if _, err := os.Stat(name); err != nil {
		if os.IsNotExist(err) {
			return false
		}
	}
	return true
}


func showDuration(duration time.Duration) string {
	if duration < 0 {
		return "Overdue!"
	}
	seconds := int64(duration.Seconds()) % 60
	minutes := int64(duration.Minutes()) % 60
	hours := int64(duration.Hours()) % 24
	return fmt.Sprintf("%02d:%02d:%02d", hours, minutes, seconds)
}