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
	"github.com/wagoodman/bashful/runtime"
	"github.com/wagoodman/bashful/utils"
	"io/ioutil"
	"path/filepath"
)

// bundleCmd represents the bundle command
var bundleCmd = &cobra.Command{
	Use:   "bundle",
	Short: "Bundle yaml and referenced resources into a single executable (experimental)",
	Long:  `Bundle yaml and referenced resources into a single executable (experimental)`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {

		cli := config.Cli{
			YamlPath: args[0],
		}

		bundlePath := filepath.Base(cli.YamlPath[0:len(cli.YamlPath)-len(filepath.Ext(cli.YamlPath))]) + ".bundle"

		yamlString, err := ioutil.ReadFile(cli.YamlPath)
		utils.CheckError(err, "Unable to read yaml config.")

		fmt.Print("\033[?25l") // hide cursor
		Bundle(yamlString, bundlePath, cli)
	},
}

func init() {
	rootCmd.AddCommand(bundleCmd)
}

func Bundle(yamlString []byte, outputPath string, cli config.Cli) {

	yamlString, err := ioutil.ReadFile(cli.YamlPath)
	utils.CheckError(err, "Unable to read yaml Config.")

	client, err := runtime.NewClientFromConfig(yamlString, &cli)
	if err != nil {
		utils.ExitWithErrorMessage(err.Error())
	}

	fmt.Println(utils.Bold("Bundling " + cli.YamlPath + " to " + outputPath))

	client.Bundle(cli.YamlPath, outputPath)

}
