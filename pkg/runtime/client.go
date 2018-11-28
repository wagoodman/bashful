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
	"bytes"
	"fmt"
	"github.com/wagoodman/bashful/pkg/config"
	"github.com/wagoodman/bashful/pkg/log"
	"github.com/wagoodman/bashful/utils"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"text/template"
)

func NewClientFromYaml(yamlString []byte, cli *config.Cli) (*Client, error) {
	cfg, err := config.NewConfig(yamlString, cli)
	if err != nil {
		return nil, err
	}
	return NewClientFromConfig(cfg)
}

func NewClientFromConfig(cfg *config.Config) (*Client, error) {

	return &Client{
		Config:   cfg,
		Executor: newExecutor(cfg),
	}, nil
}

func (client *Client) AddEventHandler(handler EventHandler) {
	client.Executor.addEventHandler(handler)
}

func (client *Client) Run() error {

	for _, task := range client.Executor.Tasks {
		if task.requiresSudoPassword() {
			sudoPassword = utils.GetSudoPasswd()
			break
		}
	}

	assetManager := NewDownloader(client.Executor.Tasks, client.Config.DownloadCachePath, client.Config.Options.MaxParallelCmds)
	assetManager.Download()

	client.Executor.estimateRuntime()
	client.Executor.run()

	if len(client.Executor.Statistics.Failed) > 0 {
		var buffer bytes.Buffer
		buffer.WriteString(utils.Red(" ...Some Tasks failed, see below for details.\n"))

		for _, task := range client.Executor.Statistics.Failed {

			buffer.WriteString("\n")
			buffer.WriteString(utils.Bold(utils.Red("• Failed task: ")) + utils.Bold(task.Config.Name) + "\n")
			buffer.WriteString(utils.Red("  ├─ command: ") + task.Config.CmdString + "\n")
			buffer.WriteString(utils.Red("  ├─ return code: ") + strconv.Itoa(task.Command.ReturnCode) + "\n")
			buffer.WriteString(utils.Red("  └─ stderr: ") + task.Command.errorBuffer.String() + "\n")

		}
		log.LogToMain(buffer.String(), "")

		// we may not show the error report, but we always log it.
		if client.Config.Options.ShowFailureReport {
			fmt.Print(buffer.String())
		}

	}

	if len(client.Executor.Statistics.Failed) > 0 {
		return fmt.Errorf("failed Tasks discovered")
	}

	return nil
}

func (client *Client) Bundle(userYamlPath, outputPath string) error {
	assetManager := NewDownloader(client.Executor.Tasks, client.Config.DownloadCachePath, client.Config.Options.MaxParallelCmds)
	assetManager.Download()

	archivePath := "bundle.tar.gz"

	bashfulPath, err := os.Executable()
	utils.CheckError(err, "Could not find path to bashful")

	archive := NewArchive(archivePath)

	for _, path := range []string{userYamlPath, bashfulPath} {
		err = archive.Archive(path, false)
		utils.CheckError(err, "Unable to add '"+path+"' to bundle")
	}

	for _, path := range append([]string{client.Config.CachePath}, client.Config.Options.Bundle...) {
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
	utils.CheckError(err, "Failed to parse Execute template")
	err = tmpl.Execute(&buff, values)
	utils.CheckError(err, "Failed to render Execute template")

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
