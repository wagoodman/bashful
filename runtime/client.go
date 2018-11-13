// Copyright © 2018 Alex Goodman
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package runtime

import (
	"github.com/wagoodman/bashful/config"
	"time"
	"bytes"
	"github.com/wagoodman/bashful/utils"
	"os"
	"path/filepath"
	"io"
	"text/template"
	"strconv"
	"github.com/wagoodman/bashful/log"
	"fmt"
)

func NewClientFromConfig(yamlString []byte) *Client {
	config.ParseConfig(yamlString)
	return NewClient(config.Config.TaskConfigs, config.Config.Options)

}

func NewClient(taskConfigs []config.TaskConfig, options config.OptionsConfig) *Client {
	startTime = time.Now()

	// initialize Tasks with default values
	var tasks []*Task
	for _, taskConfig := range taskConfigs {
		nextDisplayIdx = 0

		// finalize task by appending to the set of final Tasks
		task := NewTask(taskConfig, nextDisplayIdx, "")
		tasks = append(tasks, task)
	}

	// TODO: move this to the Executor?
	// now that all Tasks have been inflated, set the total eta
	for _, task := range tasks {
		config.Config.TotalEtaSeconds += task.EstimateRuntime()
	}

	return &Client{
		Options:     options,
		TaskConfigs: taskConfigs,
		Executor:    newExecutor(tasks),
	}
}

func (client *Client) AddEventHandler(handler EventHandler) {
	client.Executor.addEventHandler(handler)
}

func (client *Client) Run() error {
	for _, task := range client.Executor.Tasks {
		if task.requiresSudoPasswd() {
			sudoPassword = utils.GetSudoPasswd()
			break
		}
	}

	DownloadAssets(client.Executor.Tasks)
	client.Executor.run()

	if client.Options.ShowSummaryFooter {
		// todo: add footer update via Executor stats
		message := ""
		GetScreen().ResetFrame(0, false, true)
		if len(client.Executor.FailedTasks) > 0 {
			if config.Config.Options.LogPath != "" {
				message = utils.Bold(" See log for details (" + config.Config.Options.LogPath + ")")
			}
			GetScreen().DisplayFooter(footer(StatusError, message, client.Executor))
		} else {
			GetScreen().DisplayFooter(footer(StatusSuccess, message, client.Executor))
		}
	}

	if len(client.Executor.FailedTasks) > 0 {
		var buffer bytes.Buffer
		buffer.WriteString(utils.Red(" ...Some Tasks failed, see below for details.\n"))

		for _, task := range client.Executor.FailedTasks {

			buffer.WriteString("\n")
			buffer.WriteString(utils.Bold(utils.Red("• Failed task: ")) + utils.Bold(task.Config.Name) + "\n")
			buffer.WriteString(utils.Red("  ├─ command: ") + task.Config.CmdString + "\n")
			buffer.WriteString(utils.Red("  ├─ return code: ") + strconv.Itoa(task.Command.ReturnCode) + "\n")
			buffer.WriteString(utils.Red("  └─ stderr: ") + task.ErrorBuffer.String() + "\n")

		}
		log.LogToMain(buffer.String(), "")

		// we may not show the error report, but we always log it.
		if config.Config.Options.ShowFailureReport {
			fmt.Print(buffer.String())
		}

	}

	if len(client.Executor.FailedTasks) > 0 {
		return fmt.Errorf("failed Tasks discovered")
	}

	return nil
}


func (client *Client) Bundle(userYamlPath, outputPath string) error {
	DownloadAssets(client.Executor.Tasks)

	archivePath := "bundle.tar.gz"

	bashfulPath, err := os.Executable()
	utils.CheckError(err, "Could not find path to bashful")

	archive := NewArchive(archivePath)

	for _, path := range []string{userYamlPath, bashfulPath} {
		err = archive.Archive(path, false)
		utils.CheckError(err, "Unable to add '"+path+"' to bundle")
	}

	for _, path := range append([]string{config.Config.CachePath}, config.Config.Options.Bundle...) {
		err = archive.Archive(path, true)
		utils.CheckError(err, "Unable to add '"+path+"' to bundle")
	}

	archive.Close()

	execute := `#!/bin/bash
set -eu
export TMPDIR=$(mktemp -d /tmp/bashful.XXXXXX)
ARCHIVE=$(awk '/^__BASHFUL_ARCHIVE__/ {print NR + 1; exit 0; }' $0)

tail -n+$ARCHIVE $0 | tar -xz -C $TMPDIR

pushd $TMPDIR > /dev/null
./bashful run {{.Runyaml}} $*
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
	utils.CheckError(err, "Failed to parse execute template")
	err = tmpl.Execute(&buff, values)
	utils.CheckError(err, "Failed to render execute template")

	runnerFh, err := os.Create(outputPath)
	utils.CheckError(err, "Unable to create runner executable file")
	defer runnerFh.Close()

	_, err = runnerFh.Write(buff.Bytes())
	utils.CheckError(err, "Unable to write bootstrap script to runner executable file")

	archiveFh, err := os.Open(archivePath)
	utils.CheckError(err, "Unable to open payload file")
	defer archiveFh.Close()
	defer os.Remove(archivePath)

	_, err = io.Copy(runnerFh, archiveFh)
	utils.CheckError(err, "Unable to write payload to runner executable file")

	err = os.Chmod(outputPath, 0755)
	utils.CheckError(err, "Unable to change runner permissions")

	return nil
}

