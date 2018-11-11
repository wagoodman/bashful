package core

import (
	"crypto/md5"
	"fmt"
	"io"
	"net/url"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/cavaliercoder/grab"
	"github.com/deckarep/golang-set"
	"github.com/dustin/go-humanize"
	"github.com/gosuri/uiprogress"
	"github.com/wagoodman/bashful/config"
	"github.com/wagoodman/bashful/utils"
	"github.com/wagoodman/bashful/log"
)

var registry struct {
	urlToRequest  map[string]*grab.Request
	requestToTask map[*grab.Request][]*Task
	urltoFilename map[string]string
}

func getFilename(urlStr string) string {
	uri, err := url.Parse(urlStr)
	utils.CheckError(err, "Unable to parse URI")

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
			urlStr = getFilename(urlStr)
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
func AddRequest(task *Task) {
	if task.Config.URL != "" {
		request, ok := registry.urlToRequest[task.Config.URL]
		if !ok {
			// never seen this url before
			filepath := path.Join(config.Config.DownloadCachePath, getFilename(task.Config.URL))

			if _, err := os.Stat(filepath); err == nil {
				// the asset already exists, skip (unless it has an unexpected checksum)
				if task.Config.Md5 != "" {

					actualHash := md5OfFile(filepath)
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

func md5OfFile(filepath string) string {
	f, err := os.Open(filepath)
	utils.CheckError(err, "File does not exist: "+filepath)
	defer f.Close()

	h := md5.New()
	_, err = io.Copy(h, f)
	utils.CheckError(err, "Could not calculate md5 checksum of "+filepath)

	return fmt.Sprintf("%x", h.Sum(nil))
}

// DownloadAssets fetches all assets for the given task
func DownloadAssets(tasks []*Task) {
	registry.urlToRequest = make(map[string]*grab.Request)
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
			utils.ExitWithErrorMessage("Provided two different urls with the same filename!")
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

	if len(allRequests) == 0 {
		log.LogToMain("No assets to download", majorFormat)
		return
	}

	fmt.Println(utils.Bold("Downloading referenced assets"))
	log.LogToMain("Downloading referenced assets", majorFormat)

	uiprogress.Empty = ' '
	uiprogress.Fill = '|'
	uiprogress.Head = ' '
	uiprogress.LeftEnd = '|'
	uiprogress.RightEnd = '|'

	uiprogress.Start()
	respch := client.DoBatch(config.Config.Options.MaxParallelCmds, allRequests...)
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
			log.LogToMain(fmt.Sprintf(utils.Red("Failed to download '%s': %s"), response.Request.URL(), err.Error()), errorFormat)
			foundFailedAsset = true
		}
		if response.HTTPResponse.StatusCode > 399 || response.HTTPResponse.StatusCode < 200 {
			log.LogToMain(fmt.Sprintf(utils.Red("Failed to download '%s': Bad HTTP response code (%d)"), response.Request.URL(), response.HTTPResponse.StatusCode), errorFormat)
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
					utils.ExitWithErrorMessage("Asset '" + filepath + "' checksum failed. Expected: " + task.Config.Md5 + " Got: " + actualHash)
				}
			}
		}
	}

	if foundFailedAsset {
		utils.ExitWithErrorMessage("Asset download failed")
	}

	log.LogToMain("Asset download complete", majorFormat)
}
