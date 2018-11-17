package utils

import (
	"bytes"
	"crypto/md5"
	"encoding/gob"
	"errors"
	"fmt"
	"github.com/howeyc/gopass"
	color "github.com/mgutz/ansi"
	"io"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

var (
	Purple = color.ColorFunc("magenta+h")
	Red    = color.ColorFunc("red+h")
	Blue   = color.ColorFunc("blue+h")
	Bold   = color.ColorFunc("default+b")
)

// MinMax returns the min and max values from an array of float64 values
func MinMax(array []float64) (float64, float64, error) {
	if len(array) == 0 {
		return 0, 0, errors.New("no min/max of empty array")
	}
	var max = array[0]
	var min = array[0]
	for _, value := range array {
		if max < value {
			max = value
		}
		if min > value {
			min = value
		}
	}
	return min, max, nil
}

// RemoveOneValue removes the first matching value from the given array of float64 values
func RemoveOneValue(slice []float64, value float64) []float64 {
	for index, arrValue := range slice {
		if arrValue == value {
			return append(slice[:index], slice[index+1:]...)
		}
	}
	return slice
}

func VisualLength(str string) int {
	inEscapeSeq := false
	length := 0

	for _, r := range str {
		switch {
		case inEscapeSeq:
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
				inEscapeSeq = false
			}
		case r == '\x1b':
			inEscapeSeq = true
		default:
			length++
		}
	}

	return length
}

func TrimToVisualLength(message string, length int) string {
	for VisualLength(message) > length && len(message) > 1 {
		message = message[:len(message)-1]
	}
	return message
}

func ExitWithErrorMessage(msg string) {
	cleanup()
	fmt.Fprintln(os.Stderr, Red(msg))
	os.Exit(1)
}

func Exit(rc int) {
	cleanup()
	os.Exit(rc)
}

func CheckError(err error, message string) {
	if err != nil {
		fmt.Println(Red("Error:"))
		_, file, line, _ := runtime.Caller(1)
		fmt.Println("Line:", line, "\tFile:", file, "\n", err)
		ExitWithErrorMessage(message)
	}
}

// TODO: THIS NEEDS TO BE RETHOUGHT
func cleanup() {
	// // stop any running tasks
	// for _, task := range AllTasks {
	// 	task.Kill()
	// }
	//
	// // move the cursor past the used screen realestate
	// GetScreen().MovePastFrame(true)
	//
	// // show the cursor again
	// fmt.Print("\033[?25h") // show cursor
}

func DoesFileExist(name string) bool {
	if _, err := os.Stat(name); err != nil {
		if os.IsNotExist(err) {
			return false
		}
	}
	return true
}

func ShowDuration(duration time.Duration) string {
	if duration < 0 {
		return "Overdue!"
	}
	seconds := int64(duration.Seconds()) % 60
	minutes := int64(duration.Minutes()) % 60
	hours := int64(duration.Hours()) % 24
	return fmt.Sprintf("%02d:%02d:%02d", hours, minutes, seconds)
}

// todo: return error and have caller handle
func GetSudoPasswd() string {
	var stdOut bytes.Buffer
	var password string

	// test if a password is even required for sudo
	cmd := exec.Command("/bin/sh", "-c", "sudo -Sn /bin/true")
	cmd.Stderr = &stdOut
	err := cmd.Run()
	requiresPassword := stdOut.String() == "sudo: a password is required\n"

	if requiresPassword {
		fmt.Print("[bashful] sudo password required: ")
		password, err := gopass.GetPasswd()
		CheckError(err, "Could get sudo password from user.")

		// test the given password
		cmdTest := exec.Command("/bin/sh", "-c", "sudo -S /bin/true")
		cmdTest.Stdin = strings.NewReader(string(password) + "\n")
		err = cmdTest.Run()
		if err != nil {
			ExitWithErrorMessage("Given sudo password did not work.")
		}
	} else {
		CheckError(err, "Could not determine sudo access for user.")
	}

	return password
}

// Save encodes a generic object via Gob to the given file path
func Save(path string, object interface{}) error {
	file, err := os.Create(path)
	if err == nil {
		encoder := gob.NewEncoder(file)
		encoder.Encode(object)
	}
	file.Close()
	return err
}

// Load decodes via Gob the contents of the given file to an object
func Load(path string, object interface{}) error {
	file, err := os.Open(path)
	if err == nil {
		decoder := gob.NewDecoder(file)
		err = decoder.Decode(object)
	}
	file.Close()
	return err
}

// todo: return error and have caller handle
func GetFilenameFromUrl(urlStr string) string {
	uri, err := url.Parse(urlStr)
	CheckError(err, "Unable to parse URI")

	pathElements := strings.Split(uri.Path, "/")

	return pathElements[len(pathElements)-1]
}

// todo: return error and have caller handle
func Md5OfFile(filepath string) string {
	f, err := os.Open(filepath)
	CheckError(err, "File does not exist: "+filepath)
	defer f.Close()

	h := md5.New()
	_, err = io.Copy(h, f)
	CheckError(err, "Could not calculate md5 checksum of "+filepath)

	return fmt.Sprintf("%x", h.Sum(nil))
}
