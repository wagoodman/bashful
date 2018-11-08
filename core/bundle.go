package core

import (
	"text/template"
	"io/ioutil"
	"fmt"
	"os"
	"bytes"
	"path/filepath"
	"io"
	"github.com/wagoodman/bashful/utils"
	"github.com/wagoodman/bashful/config"
	"github.com/wagoodman/bashful/task"
)

func Bundle(userYamlPath, outputPath string) {
	archivePath := "bundle.tar.gz"

	yamlString, err := ioutil.ReadFile(userYamlPath)
	utils.CheckError(err, "Unable to read yaml Config.")

	config.ParseConfig(yamlString)
	AllTasks := task.CreateTasks()

	DownloadAssets(AllTasks)

	fmt.Println(bold("Bundling " + userYamlPath + " to " + outputPath))

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

}
