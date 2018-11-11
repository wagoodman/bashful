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

package core

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"github.com/wagoodman/bashful/utils"
)

type Archiver interface {
	Archive(srcPath string, preservePath bool) error
	Close()
}

func NewArchive(dest string) Archiver {
	fw, err := os.Create(dest)
	utils.CheckError(err, "Could not create archive file")
	gw := gzip.NewWriter(fw)
	tw := tar.NewWriter(gw)
	return &archive{
		outputFile: fw,
		gzipWriter: gw,
		tarWriter:  tw,
	}
}

type archive struct {
	outputFile *os.File
	gzipWriter *gzip.Writer
	tarWriter  *tar.Writer
}

func (archiver *archive) Close() {
	archiver.tarWriter.Close()
	archiver.gzipWriter.Close()
	archiver.outputFile.Close()
}

func isDir(pth string) (bool, error) {
	fi, err := os.Stat(pth)
	if err != nil {
		return false, err
	}

	return fi.Mode().IsDir(), nil
}

func (archiver *archive) Archive(srcPath string, preservePath bool) error {
	absPath, err := filepath.Abs(srcPath)
	if err != nil {
		return err
	}

	isDirectory, err := isDir(srcPath)
	utils.CheckError(err, "Could not determine if this is a directory.")

	if isDirectory || !preservePath {
		err = filepath.Walk(absPath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			var relative string
			if os.IsPathSeparator(srcPath[len(srcPath)-1]) {
				relative, err = filepath.Rel(absPath, path)
			} else {
				relative, err = filepath.Rel(filepath.Dir(absPath), path)
			}

			relative = filepath.ToSlash(relative)

			if err != nil {
				return err
			}

			return archiver.addTarFile(path, relative)
		})
	} else {
		fields := strings.Split(srcPath, string(os.PathSeparator))
		for idx := range fields {
			path := strings.Join(fields[:idx+1], string(os.PathSeparator))
			err := archiver.addTarFile(path, path)
			utils.CheckError(err, "Unable to archive file")
		}
	}

	return err
}

func (archiver *archive) addTarFile(path, name string) error {
	if strings.Contains(path, "..") {
		return errors.New("Path cannot contain a relative marker of '..': " + path)
	}
	fi, err := os.Lstat(path)
	if err != nil {
		return err
	}

	link := ""
	if fi.Mode()&os.ModeSymlink != 0 {
		if link, err = os.Readlink(path); err != nil {
			return err
		}
	}

	hdr, err := tar.FileInfoHeader(fi, link)
	if err != nil {
		return err
	}

	if fi.IsDir() && !os.IsPathSeparator(name[len(name)-1]) {
		name = name + "/"
	}

	if hdr.Typeflag == tar.TypeReg && name == "." {
		// archiving a single file
		hdr.Name = filepath.ToSlash(filepath.Base(path))
	} else {
		hdr.Name = filepath.ToSlash(name)
	}

	if err := archiver.tarWriter.WriteHeader(hdr); err != nil {
		return err
	}

	if hdr.Typeflag == tar.TypeReg {
		file, err := os.Open(path)
		if err != nil {
			return err
		}

		defer file.Close()

		_, err = io.Copy(archiver.tarWriter, file)
		if err != nil {
			return err
		}
	}

	return nil
}
