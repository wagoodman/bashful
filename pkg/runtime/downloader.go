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
	"fmt"
	"os"
	"path"
	"sync"
	"time"

	mapset "github.com/deckarep/golang-set"

	"github.com/cavaliercoder/grab"
	"github.com/dustin/go-humanize"
	"github.com/gosuri/uiprogress"
	"github.com/wagoodman/bashful/pkg/log"
	"github.com/wagoodman/bashful/utils"
)

type downloader struct {
	downloadPath  string
	maxParallel   int
	urlToRequest  map[string]*grab.Request
	requestToTask map[*grab.Request][]*Task
	urltoFilename map[string]string
}

func NewDownloader(tasks []*Task, downloadPath string, maxParallel int) *downloader {
	if _, err := os.Stat(downloadPath); os.IsNotExist(err) {
		os.Mkdir(downloadPath, 0755)
	}

	registry := &downloader{
		downloadPath:  downloadPath,
		maxParallel:   maxParallel,
		urlToRequest:  make(map[string]*grab.Request),
		requestToTask: make(map[*grab.Request][]*Task),
		urltoFilename: make(map[string]string),
	}

	// gather all possible requests
	for _, task := range tasks {
		registry.AddRequest(task)
		for _, subTask := range task.Children {
			registry.AddRequest(subTask)
		}
	}

	// ensure there are no files with the same name, if so, correct this
	targetFilenames := mapset.NewSet()
	for _, filename := range registry.urltoFilename {
		if targetFilenames.Contains(filename) {
			utils.ExitWithErrorMessage("Provided two different urls with the same filename!")
		}
		targetFilenames.Add(filename)
	}

	return registry
}

func (registry *downloader) monitorDownload(requests map[*grab.Request][]*Task, response *grab.Response, waiter *sync.WaitGroup) {
	bar := uiprogress.AddBar(100)
	bar.AppendFunc(func(b *uiprogress.Bar) string {

		size := response.Size
		if size < 0 {
			size = response.BytesComplete()
		}

		if response.IsComplete() {
			if response.HTTPResponse.StatusCode > 399 || response.HTTPResponse.StatusCode < 200 {
				return utils.Red("Failed!")
			}
			return fmt.Sprintf("%7s [%v]",
				"100.00%",
				humanize.Bytes(uint64(size)))
		}

		progressValue := 100 * response.Progress()
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
	})
	bar.PrependFunc(func(b *uiprogress.Bar) string {
		urlStr := requests[response.Request][0].Config.URL
		if len(urlStr) > 25 {
			urlStr = utils.GetFilenameFromUrl(urlStr)
		}
		if len(urlStr) > 25 {
			urlStr = "..." + urlStr[len(urlStr)-20:]
		}
		return fmt.Sprintf("%-25s", urlStr)
	})
	t := time.NewTicker(100 * time.Millisecond)
	defer t.Stop()
Loop:
	for {
		select {
		case <-t.C:
			bar.Set(int(100 * response.Progress()))

		case <-response.Done:
			bar.Set(100)
			break Loop
		}
	}

	// rename file to match the last part of the url
	expectedFilepath := registry.urltoFilename[response.Request.URL().String()]
	if response.Filename != expectedFilepath {
		err := os.Rename(response.Filename, expectedFilepath)
		utils.CheckError(err, "Unable to rename downloaded asset: "+response.Filename)
	}

	// ensure the asset is executable
	err := os.Chmod(expectedFilepath, 0755)
	utils.CheckError(err, "Unable to make asset executable: "+expectedFilepath)

	// update all Tasks using this asset to use the final filepath
	for _, task := range registry.requestToTask[response.Request] {
		task.UpdateExec(expectedFilepath)
	}

	waiter.Done()

}

// AddRequest extracts all URLS configured for a given task (does not examine child Tasks) and queues them for download
func (registry *downloader) AddRequest(task *Task) {
	if task.Config.URL != "" {
		request, ok := registry.urlToRequest[task.Config.URL]
		if !ok {
			// never seen this url before
			filepath := path.Join(registry.downloadPath, utils.GetFilenameFromUrl(task.Config.URL))

			if _, err := os.Stat(filepath); err == nil {
				// the asset already exists, skip (unless it has an unexpected checksum)
				if task.Config.Md5 != "" {

					actualHash := utils.Md5OfFile(filepath)
					if task.Config.Md5 != actualHash {
						utils.ExitWithErrorMessage("Already downloaded asset '" + filepath + "' checksum failed. Expected: " + task.Config.Md5 + " Got: " + actualHash)
					}

				}
				task.UpdateExec(filepath)
				return
			}
			// the asset has not already been downloaded
			request, _ = grab.NewRequest(filepath, task.Config.URL)

			// workaround for https://github.com/cavaliercoder/grab/issues/25, allow the ability to follow 302s
			//request.IgnoreBadStatusCodes = true

			registry.urltoFilename[task.Config.URL] = filepath
			registry.urlToRequest[task.Config.URL] = request
		}
		registry.requestToTask[request] = append(registry.requestToTask[request], task)
	}
}

// DownloadAssets fetches all assets for the given task
func (registry *downloader) Download() {

	client := grab.NewClient()

	allRequests := make([]*grab.Request, len(registry.requestToTask))
	i := 0
	for k := range registry.requestToTask {
		allRequests[i] = k
		i++
	}

	if len(allRequests) == 0 {
		log.LogToMain("No assets to download", log.StyleMajor)
		return
	}

	fmt.Println(utils.Bold("Downloading referenced assets"))
	log.LogToMain("Downloading referenced assets", log.StyleMajor)

	uiprogress.Empty = ' '
	uiprogress.Fill = '|'
	uiprogress.Head = ' '
	uiprogress.LeftEnd = '|'
	uiprogress.RightEnd = '|'

	uiprogress.Start()
	respch := client.DoBatch(registry.maxParallel, allRequests...)
	var waiter sync.WaitGroup
	var responses []*grab.Response
	for response := range respch {

		waiter.Add(1)
		responses = append(responses, response)
		go registry.monitorDownload(registry.requestToTask, response, &waiter)
	}

	waiter.Wait()
	uiprogress.Stop()

	// verify no download errors
	foundFailedAsset := false
	for _, response := range responses {
		if err := response.Err(); err != nil {
			log.LogToMain(fmt.Sprintf(utils.Red("Failed to download '%s': %s"), response.Request.URL(), err.Error()), log.StyleError)
			foundFailedAsset = true
		}
		if response.HTTPResponse.StatusCode > 399 || response.HTTPResponse.StatusCode < 200 {
			log.LogToMain(fmt.Sprintf(utils.Red("Failed to download '%s': Bad HTTP response code (%d)"), response.Request.URL(), response.HTTPResponse.StatusCode), log.StyleError)
			foundFailedAsset = true
		}
	}

	// verify provided md5 checksums are valid
	for _, response := range responses {
		for _, task := range registry.requestToTask[response.Request] {
			if task.Config.Md5 != "" {
				filepath := registry.urltoFilename[response.Request.URL().String()]
				actualHash := utils.Md5OfFile(filepath)
				if task.Config.Md5 != actualHash {
					utils.ExitWithErrorMessage("Asset '" + filepath + "' checksum failed. Expected: " + task.Config.Md5 + " Got: " + actualHash)
				}
			}
		}
	}

	if foundFailedAsset {
		utils.ExitWithErrorMessage("Asset download failed")
	}

	log.LogToMain("Asset download complete", log.StyleMajor)
}
