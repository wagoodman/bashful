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

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/wagoodman/bashful/config"
	"github.com/wagoodman/bashful/log"
	"github.com/wagoodman/bashful/runtime"
	"github.com/wagoodman/bashful/runtime/handler"
	"github.com/wagoodman/bashful/utils"
	"io/ioutil"
	"math/rand"
	"strings"
	"time"
)

var tags, onlyTags string

// runCmd represents the run command
var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Execute the given yaml file with bashful",
	Long:  `Execute the given yaml file with bashful`,
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {

		userYamlPath := args[0]
		if len(args) > 1 {
			config.Config.Cli.Args = args[1:]
		} else {
			config.Config.Cli.Args = []string{}
		}

		if tags != "" && onlyTags != "" {
			utils.ExitWithErrorMessage("Options 'tags' and 'only-tags' are mutually exclusive.")
		}

		for _, value := range strings.Split(tags, ",") {
			if value != "" {
				config.Config.Cli.RunTags = append(config.Config.Cli.RunTags, value)
			}
		}

		for _, value := range strings.Split(onlyTags, ",") {
			if value != "" {
				config.Config.Cli.ExecuteOnlyMatchedTags = true
				config.Config.Cli.RunTags = append(config.Config.Cli.RunTags, value)
			}
		}

		yamlString, err := ioutil.ReadFile(userYamlPath)
		utils.CheckError(err, "Unable to read yaml config.")

		fmt.Print("\033[?25l") // hide cursor
		Run(yamlString)

	},
}

func init() {
	rootCmd.AddCommand(runCmd)

	runCmd.Flags().StringVar(&tags, "tags", "", "A comma delimited list of matching task tags. If a task's tag matches *or if it is not tagged* then it will be executed (also see --only-tags)")
	runCmd.Flags().StringVar(&onlyTags, "only-tags", "", "A comma delimited list of matching task tags. A task will only be executed if it has a matching tag")
}

func Run(yamlString []byte) {
	var err error

	client := runtime.NewClientFromConfig(yamlString)

	updateInterval := 150 * time.Millisecond
	if config.Config.Options.UpdateInterval > 150 {
		updateInterval = time.Duration(config.Config.Options.UpdateInterval) * time.Millisecond
	}
	if config.Config.Options.SingleLineDisplay {
		client.AddEventHandler(handler.NewCompressedUI())
	} else {
		client.AddEventHandler(handler.NewVerticalUI(updateInterval))
	}
	client.AddEventHandler(handler.NewTaskLogger())
	client.AddEventHandler(handler.NewSimpleLogger())

	rand.Seed(time.Now().UnixNano())

	tagInfo := ""
	if len(config.Config.Cli.RunTags) > 0 {
		if config.Config.Cli.ExecuteOnlyMatchedTags {
			tagInfo = " only matching tags: "
		} else {
			tagInfo = " non-tagged and matching tags: "
		}
		tagInfo += strings.Join(config.Config.Cli.RunTags, ", ")
	}

	fmt.Println(utils.Bold("Running " + tagInfo))
	log.LogToMain("Running "+tagInfo, log.StyleMajor)

	failedTasksErr := client.Run()
	log.LogToMain("Complete", log.StyleMajor)

	err = config.Save(config.Config.EtaCachePath, &config.Config.CommandTimeCache)
	utils.CheckError(err, "Unable to save command eta cache.")

	log.LogToMain("Exiting", "")

	if failedTasksErr != nil {
		utils.Exit(1)
	}
}
