package core

import (
	"text/template"
	"io/ioutil"
	"fmt"
	"os"
	"bytes"
	"path/filepath"
	"io"
)

func Bundle(userYamlPath, outputPath string) {
	archivePath := "bundle.tar.gz"

	yamlString, err := ioutil.ReadFile(userYamlPath)
	CheckError(err, "Unable to read yaml Config.")

	ParseConfig(yamlString)
	allTasks := CreateTasks()

	DownloadAssets(allTasks)

	fmt.Println(bold("Bundling " + userYamlPath + " to " + outputPath))

	bashfulPath, err := os.Executable()
	CheckError(err, "Could not find path to bashful")

	archive := NewArchive(archivePath)

	for _, path := range []string{userYamlPath, bashfulPath} {
		err = archive.Archive(path, false)
		CheckError(err, "Unable to add '"+path+"' to bundle")
	}

	for _, path := range append([]string{Config.CachePath}, Config.Options.Bundle...) {
		err = archive.Archive(path, true)
		CheckError(err, "Unable to add '"+path+"' to bundle")
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
	CheckError(err, "Failed to parse execute template")
	err = tmpl.Execute(&buff, values)
	CheckError(err, "Failed to render execute template")

	runnerFh, err := os.Create(outputPath)
	CheckError(err, "Unable to create runner executable file")
	defer runnerFh.Close()

	_, err = runnerFh.Write(buff.Bytes())
	CheckError(err, "Unable to write bootstrap script to runner executable file")

	archiveFh, err := os.Open(archivePath)
	CheckError(err, "Unable to open payload file")
	defer archiveFh.Close()
	defer os.Remove(archivePath)

	_, err = io.Copy(runnerFh, archiveFh)
	CheckError(err, "Unable to write payload to runner executable file")

	err = os.Chmod(outputPath, 0755)
	CheckError(err, "Unable to change runner permissions")

}