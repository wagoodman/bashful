package main

import (
	"github.com/cavaliercoder/grab"
	"github.com/gosuri/uiprogress"
	"github.com/dustin/go-humanize"
	"github.com/deckarep/golang-set"
	"fmt"
	"time"
	"sync"
	"net/url"
	"strings"
	"os"
	"path"
	"io"
	"crypto/md5"
)

var registry struct {
	urlToRequest   map[string]*grab.Request
	requestToTask  map[*grab.Request][]*Task
	urltoFilename  map[string]string
}

func getFilename(urlStr string) string {
	uri, err := url.Parse(urlStr)
	CheckError(err, "Unable to parse URI")

	pathElements := strings.Split(uri.Path, "/")

	return pathElements[len(pathElements)-1]
}

func monitorDownload(requests map[*grab.Request][]*Task, response *grab.Response, waiter *sync.WaitGroup) {
	bar := uiprogress.AddBar(100)
	bar.AppendFunc(func(b *uiprogress.Bar) string {

		size := response.Size
		if size < 0 {
			size = response.BytesComplete()
		}

		if response.IsComplete() {
			if response.HTTPResponse.StatusCode > 399 || response.HTTPResponse.StatusCode < 200 {
				return red("Failed!")
			} else {
				return fmt.Sprintf("%7s [%v]",
					"100.00%",
					humanize.Bytes(uint64(size)))
			}
		} else {

			progressValue := 100*response.Progress()
			var progress string
			if progressValue > 100 || progressValue < 0 {
				progress = "???%"
			} else {
				progress = fmt.Sprintf("%.2f%%", progressValue)
			}

			return fmt.Sprintf("%7s [%v / %v]",
				progress,
				humanize.Bytes(uint64(response.BytesComplete())),
				humanize.Bytes(uint64(size)))
		}

	})
	bar.PrependFunc(func(b *uiprogress.Bar) string {
		urlStr := requests[response.Request][0].Config.Url
		if len(urlStr) > 25 {
			urlStr = getFilename(urlStr)
		}
		if len(urlStr) > 25 {
			urlStr = "..."+urlStr[len(urlStr)-20:]
		}
		return fmt.Sprintf("%-25s", urlStr)
	})
	t := time.NewTicker(100 * time.Millisecond)
	defer t.Stop()
	Loop: for {
		select {
		case <-t.C:
			bar.Set(int(100*response.Progress()))

		case <-response.Done:
			bar.Set(100)
			break Loop
		}
	}

	// rename file to match the last part of the url
	expectedFilepath := registry.urltoFilename[response.Request.URL().String()]
	if response.Filename != expectedFilepath {
		err := os.Rename( response.Filename, expectedFilepath)
		CheckError(err, "Unable to rename downloaded asset: "+response.Filename)
	}

	// ensure the asset is executable
	err := os.Chmod(expectedFilepath, 0755)
	CheckError(err, "Unable to make asset executable: "+expectedFilepath)

	// update all tasks using this asset to use the final filepath
	for _, task := range registry.requestToTask[response.Request] {
		task.UpdateExec(expectedFilepath)
	}

	waiter.Done()

}

func AddRequest(task *Task) {
	if task.Config.Url != "" {
		request, ok := registry.urlToRequest[task.Config.Url]
		if !ok {
			// never seen this url before
			filepath := path.Join(config.downloadCachePath, getFilename(task.Config.Url))

			if _, err := os.Stat(filepath); err == nil {
				// the asset already exists, skip (unless it has an unexpected checksum)
				if task.Config.Md5 != "" {

					actualHash := md5OfFile(filepath)
					if task.Config.Md5 != actualHash {
						exitWithErrorMessage("Already downloaded asset '"+filepath+"' checksum failed. Expected: " +task.Config.Md5+ " Got: "+actualHash)
					}

				}
				task.UpdateExec(filepath)
				return
			} else {
				// the asset has not already been downloaded
				request, _ = grab.NewRequest(filepath, task.Config.Url)

				// workaround for https://github.com/cavaliercoder/grab/issues/25, allow the ability to follow 302s
				request.IgnoreBadStatusCodes = true

				registry.urltoFilename[task.Config.Url] = filepath
				registry.urlToRequest[task.Config.Url] = request
			}
		}
		registry.requestToTask[request] = append(registry.requestToTask[request], task)
	}
}

func md5OfFile(filepath string) string {
	f, err := os.Open(filepath)
	CheckError(err, "File does not exist: "+ filepath)
	defer f.Close()

	h := md5.New()
	_, err = io.Copy(h, f)
	CheckError(err, "Could not calculate md5 checksum of "+ filepath)

	return fmt.Sprintf("%x", h.Sum(nil))
}

func DownloadAssets(tasks []*Task) {
	registry.urlToRequest  = make(map[string]*grab.Request)
	registry.requestToTask = make(map[*grab.Request][]*Task)
	registry.urltoFilename = make(map[string]string)

	client := grab.NewClient()

	// gather all possible requests
	for _, task := range tasks {
		AddRequest(task)
		for _, subTask := range task.Children {
			AddRequest(subTask)
		}
	}

	// ensure there are no files with the same name, if so, correct this
	targetFilenames := mapset.NewSet()
	for _, filename := range registry.urltoFilename {
		if targetFilenames.Contains(filename) {
			exitWithErrorMessage("Provided two different urls with the same filename!")
		}
		targetFilenames.Add(filename)
	}

	// download
	allRequests := make([]*grab.Request, len(registry.requestToTask))
	i := 0
	for k := range registry.requestToTask {
		allRequests[i] = k
		i++
	}

	logToMain("Downloading referenced assets", MAJOR_FORMAT)

	uiprogress.Empty = ' '
	uiprogress.Fill = '|'
	uiprogress.Head = ' '
	uiprogress.LeftEnd = '|'
	uiprogress.RightEnd = '|'

	uiprogress.Start()
	respch := client.DoBatch(config.Options.MaxParallelCmds, allRequests...)
	var waiter sync.WaitGroup
	var responses []*grab.Response
	for response := range respch {
		waiter.Add(1)
		responses = append(responses, response)
		go monitorDownload(registry.requestToTask, response, &waiter)
	}

	waiter.Wait()
	uiprogress.Stop()

	// verify no download errors
	foundFailedAsset := false
	for _, response := range responses {
		if err := response.Err(); err != nil {
			logToMain(fmt.Sprintf(red("Failed to download '%s': %s"), response.Request.URL(), err.Error()), ERROR_FORMAT)
			foundFailedAsset = true
		}
		if response.HTTPResponse.StatusCode > 399 || response.HTTPResponse.StatusCode < 200 {
			logToMain(fmt.Sprintf(red("Failed to download '%s': Bad HTTP response code (%d)"), response.Request.URL(), response.HTTPResponse.StatusCode), ERROR_FORMAT)
			foundFailedAsset = true
		}
	}

	// verify provided md5 checksums are valid
	for _, response := range responses {
		for _, task := range registry.requestToTask[response.Request] {
			if task.Config.Md5 != "" {
				filepath := registry.urltoFilename[response.Request.URL().String()]
				actualHash := md5OfFile(filepath)
				if task.Config.Md5 != actualHash {
					exitWithErrorMessage("Asset '"+filepath+"' checksum failed. Expected: " +task.Config.Md5+ " Got: "+actualHash)
				}
			}
		}
	}

	if foundFailedAsset {
		exitWithErrorMessage("Asset download failed")
	}

	logToMain("Asset download complete", MAJOR_FORMAT)
}

