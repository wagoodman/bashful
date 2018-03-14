package main

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"text/template"
	"time"

	"github.com/howeyc/gopass"
	color "github.com/mgutz/ansi"
	"github.com/mholt/archiver"
	"github.com/spf13/afero"
	"github.com/urfave/cli"
	terminal "github.com/wayneashleyberry/terminal-dimensions"
)

const (
	majorFormat = "cyan+b"
	infoFormat  = "blue+b"
	errorFormat = "red+b"
)

var (
	appFs              afero.Fs
	version            = "No version provided"
	commit             = "No commit provided"
	buildTime          = "No build timestamp provided"
	allTasks           []*Task
	ticker             *time.Ticker
	exitSignaled       bool
	startTime          time.Time
	sudoPassword       string
	purple             = color.ColorFunc("magenta+h")
	red                = color.ColorFunc("red+h")
	blue               = color.ColorFunc("blue+h")
	bold               = color.ColorFunc("default+b")
	summaryTemplate, _ = template.New("summary line").Parse(` {{.Status}}    ` + color.Reset + ` {{printf "%-16s" .Percent}}` + color.Reset + ` {{.Steps}}{{.Errors}}{{.Msg}}{{.Split}}{{.Runtime}}{{.Eta}}`)
)

type summary struct {
	Status  string
	Percent string
	Msg     string
	Runtime string
	Eta     string
	Split   string
	Steps   string
	Errors  string
}

func checkError(err error, message string) {
	if err != nil {
		fmt.Println(red("Error:"))
		_, file, line, _ := runtime.Caller(1)
		fmt.Println("Line:", line, "\tFile:", file, "\n", err)
		exitWithErrorMessage(message)
	}
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

func footer(status CommandStatus, message string) string {
	var tpl bytes.Buffer
	var durString, etaString, stepString, errorString string

	if config.Options.ShowSummaryTimes {
		duration := time.Since(startTime)
		durString = fmt.Sprintf(" Runtime[%s]", showDuration(duration))

		totalEta := time.Duration(config.totalEtaSeconds) * time.Second
		remainingEta := time.Duration(totalEta.Seconds()-duration.Seconds()) * time.Second
		etaString = fmt.Sprintf(" ETA[%s]", showDuration(remainingEta))
	}

	if TaskStats.completedTasks == TaskStats.totalTasks {
		etaString = ""
	}

	if config.Options.ShowSummarySteps {
		stepString = fmt.Sprintf(" Tasks[%d/%d]", TaskStats.completedTasks, TaskStats.totalTasks)
	}

	if config.Options.ShowSummaryErrors {
		errorString = fmt.Sprintf(" Errors[%d]", TaskStats.totalFailedTasks)
	}

	// get a string with the summary line without a split gap (eta floats left)
	percentValue := (float64(TaskStats.completedTasks) * float64(100)) / float64(TaskStats.totalTasks)
	percentStr := fmt.Sprintf("%3.2f%% Complete", percentValue)

	if TaskStats.completedTasks == TaskStats.totalTasks {
		percentStr = status.Color("b") + percentStr + color.Reset
	} else {
		percentStr = color.Color(percentStr, "default+b")
	}

	summaryTemplate.Execute(&tpl, summary{Status: status.Color("i"), Percent: percentStr, Runtime: durString, Eta: etaString, Steps: stepString, Errors: errorString, Msg: message})

	// calculate a space buffer to push the eta to the right
	terminalWidth, _ := terminal.Width()
	splitWidth := int(terminalWidth) - visualLength(tpl.String())
	if splitWidth < 0 {
		splitWidth = 0
	}

	tpl.Reset()
	summaryTemplate.Execute(&tpl, summary{Status: status.Color("i"), Percent: percentStr, Runtime: bold(durString), Eta: bold(etaString), Split: strings.Repeat(" ", splitWidth), Steps: bold(stepString), Errors: bold(errorString), Msg: message})

	return tpl.String()
}

func doesFileExist(name string) bool {
	if _, err := os.Stat(name); err != nil {
		if os.IsNotExist(err) {
			return false
		}
	}
	return true
}

func bundle(userYamlPath, outputPath string) {
	archivePath := "bundle.tar.gz"

	yamlString, err := ioutil.ReadFile(userYamlPath)
	checkError(err, "Unable to read yaml config.")

	ParseConfig(yamlString)
	allTasks := CreateTasks()

	DownloadAssets(allTasks)

	fmt.Println(bold("Bundling " + userYamlPath + " to " + outputPath))

	bashfulPath, err := os.Executable()
	checkError(err, "Could not find path to bashful")
	err = archiver.TarGz.Make(archivePath, []string{userYamlPath, bashfulPath, config.CachePath})
	checkError(err, "Unable to create bundle")

	execute := `#!/bin/bash
set -eu
export TMPDIR=$(mktemp -d /tmp/bashful.XXXXXX)
ARCHIVE=$(awk '/^__BASHFUL_ARCHIVE__/ {print NR + 1; exit 0; }' $0)

tail -n+$ARCHIVE $0 | tar -xz -C $TMPDIR

pushd $TMPDIR > /dev/null
./bashful run {{.Runyaml}}
popd > /dev/null
rm -rf $TMPDIR

exit 0

__BASHFUL_ARCHIVE__
`
	var buff bytes.Buffer
	var values = struct {
		Runyaml string
	}{
		Runyaml: filepath.Base(userYamlPath),
	}

	tmpl := template.New("test")
	tmpl, err = tmpl.Parse(execute)
	checkError(err, "Failed to parse execute template")
	err = tmpl.Execute(&buff, values)
	checkError(err, "Failed to render execute template")

	runnerPath := "./runner"
	runnerFh, err := os.Create(runnerPath)
	checkError(err, "Unable to create runner executable file")
	defer runnerFh.Close()

	_, err = runnerFh.Write(buff.Bytes())
	checkError(err, "Unable to write bootstrap script to runner executable file")

	archiveFh, err := os.Open(archivePath)
	checkError(err, "Unable to open payload file")
	defer archiveFh.Close()
	defer os.Remove(archivePath)

	_, err = io.Copy(runnerFh, archiveFh)
	checkError(err, "Unable to write payload to runner executable file")

	err = os.Chmod(runnerPath, 0755)
	checkError(err, "Unable to change runner permissions")

}

func storeSudoPasswd() {
	var sout bytes.Buffer

	// check if there is a task that requires sudo
	requireSudo := false
	for _, task := range allTasks {
		if task.Config.Sudo {
			requireSudo = true
			break
		}
		for _, subTask := range task.Children {
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
		sudoPassword, err := gopass.GetPasswd()
		checkError(err, "Could get sudo password from user.")

		// test the given password
		cmdTest := exec.Command("/bin/sh", "-c", "sudo -S /bin/true")
		cmdTest.Stdin = strings.NewReader(string(sudoPassword) + "\n")
		err = cmdTest.Run()
		if err != nil {
			exitWithErrorMessage("Given sudo password did not work.")
		}
	} else {
		checkError(err, "Could not determine sudo access for user.")
	}
}

func run(yamlString []byte, environment map[string]string) []*Task {
	var err error

	exitSignaled = false
	startTime = time.Now()

	ParseConfig(yamlString)
	allTasks = CreateTasks()
	storeSudoPasswd()

	DownloadAssets(allTasks)

	rand.Seed(time.Now().UnixNano())

	if config.Options.UpdateInterval > 150 {
		ticker = time.NewTicker(time.Duration(config.Options.UpdateInterval) * time.Millisecond)
	} else {
		ticker = time.NewTicker(150 * time.Millisecond)
	}

	var failedTasks []*Task

	tagInfo := ""
	if len(config.Cli.RunTags) > 0 {
		if config.Cli.ExecuteOnlyMatchedTags {
			tagInfo = " only matching tags: "
		} else {
			tagInfo = " non-tagged and matching tags: "
		}
		tagInfo += strings.Join(config.Cli.RunTags, ", ")
	}

	fmt.Println(bold("Running " + tagInfo))
	logToMain("Running "+tagInfo, majorFormat)

	for _, task := range allTasks {
		task.Run(environment)
		failedTasks = append(failedTasks, task.failedTasks...)

		if exitSignaled {
			break
		}
	}
	logToMain("Complete", majorFormat)

	err = Save(config.etaCachePath, &config.commandTimeCache)
	checkError(err, "Unable to save command eta cache.")

	if config.Options.ShowSummaryFooter {
		message := ""
		newScreen().ResetFrame(0, false, true)
		if len(failedTasks) > 0 {
			if config.Options.LogPath != "" {
				message = bold(" See log for details (" + config.Options.LogPath + ")")
			}
			newScreen().DisplayFooter(footer(statusError, message))
		} else {
			newScreen().DisplayFooter(footer(statusSuccess, message))
		}
	}

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
		logToMain(buffer.String(), "")

		// we may not show the error report, but we always log it.
		if config.Options.ShowFailureReport {
			fmt.Print(buffer.String())
		}

	}

	return failedTasks
}

func exitWithErrorMessage(msg string) {
	cleanup()
	fmt.Fprintln(os.Stderr, red(msg))
	os.Exit(1)
}

func exit(rc int) {
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

func setup() {
	sigChannel := make(chan os.Signal, 2)
	signal.Notify(sigChannel, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		for sig := range sigChannel {
			if sig == syscall.SIGINT {
				exitWithErrorMessage(red("Keyboard Interrupt"))
			} else if sig == syscall.SIGTERM {
				exit(0)
			} else {
				exitWithErrorMessage("Unknown Signal: " + sig.String())
			}
		}
	}()
}

func main() {
	setup()
	appFs = afero.NewOsFs()
	app := cli.NewApp()
	app.Name = "bashful"
	app.Version = "Version:   " + version + "\n   Commit:    " + commit + "\n   BuildTime: " + buildTime
	app.Usage = "Takes a yaml file containing commands and bash snippits and executes each command while showing a simple (vertical) progress bar."
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:        "cache-path",
			Value:       "",
			Usage:       "The path where cached files will be stored. By default '$(pwd)/.bashful' is used.",
			Destination: &config.CachePath,
		},
	}
	app.Commands = []cli.Command{
		{
			Name:  "bundle",
			Usage: "Bundle yaml and referenced resources into a single executable",
			Action: func(cliCtx *cli.Context) error {
				if cliCtx.NArg() < 1 {
					exitWithErrorMessage("Must provide the path to a bashful yaml file")
				} else if cliCtx.NArg() > 1 {
					exitWithErrorMessage("Only one bashful yaml file can be provided at a time")
				}

				userYamlPath := cliCtx.Args().Get(0)
				bundlePath := userYamlPath + ".bundle"

				bundle(userYamlPath, bundlePath)

				return nil
			},
		},
		{
			Name:  "run",
			Usage: "Execute the given yaml file with bashful",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "tags",
					Value: "",
					Usage: "A comma delimited list of matching task tags. If a task's tag matches *or if it is not tagged* then it will be executed (also see --only-tags).",
				},
				cli.StringFlag{
					Name:  "only-tags",
					Value: "",
					Usage: "A comma delimited list of matching task tags. A task will only be executed if it has a matching tag.",
				},
			},
			Action: func(cliCtx *cli.Context) error {
				if cliCtx.NArg() < 1 {
					exitWithErrorMessage("Must provide the path to a bashful yaml file")
				}

				userYamlPath := cliCtx.Args().Get(0)
				config.Cli.Args = cliCtx.Args().Tail()

				if cliCtx.String("tags") != "" && cliCtx.String("only-tags") != "" {
					exitWithErrorMessage("Options 'tags' and 'only-tags' are mutually exclusive.")
				}

				for _, value := range strings.Split(cliCtx.String("tags"), ",") {
					if value != "" {
						config.Cli.RunTags = append(config.Cli.RunTags, value)
					}
				}

				for _, value := range strings.Split(cliCtx.String("only-tags"), ",") {
					if value != "" {
						config.Cli.ExecuteOnlyMatchedTags = true
						config.Cli.RunTags = append(config.Cli.RunTags, value)
					}
				}

				// Since this is an empty map, no env vars will be loaded explicitly into the first exec.Command
				// which will cause the current processes env vars to be loaded instead
				environment := map[string]string{}

				yamlString, err := ioutil.ReadFile(userYamlPath)
				checkError(err, "Unable to read yaml config.")

				fmt.Print("\033[?25l") // hide cursor
				failedTasks := run(yamlString, environment)

				logToMain("Exiting", "")

				exit(len(failedTasks))
				return nil
			},
		},
	}

	app.Run(os.Args)

}
