package main

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type Archiver interface {
	Archive(srcPath string, preservePath bool) error
	Close()
}

func NewArchive(dest string) Archiver {
	fw, err := os.Create(dest)
	checkError(err, "Could not create archive file")
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
	checkError(err, "Could not determine if this is a directory.")

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
			archiver.addTarFile(path, path)
		}
	}

	return err
}

func (archiver *archive) addTarFile(path, name string) error {
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
