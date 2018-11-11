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
	"github.com/spf13/cobra"
	"path/filepath"
	"io/ioutil"
	"github.com/wagoodman/bashful/utils"
	"github.com/wagoodman/bashful/config"
	"fmt"
	"github.com/wagoodman/bashful/runtime"
)

// bundleCmd represents the bundle command
var bundleCmd = &cobra.Command{
	Use:   "bundle",
	Short: "Bundle yaml and referenced resources into a single executable (experimental)",
	Long:  `Bundle yaml and referenced resources into a single executable (experimental)`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {

		userYamlPath := args[0]
		bundlePath := filepath.Base(userYamlPath[0:len(userYamlPath)-len(filepath.Ext(userYamlPath))]) + ".bundle"

		Bundle(userYamlPath, bundlePath)
	},
}

func init() {
	rootCmd.AddCommand(bundleCmd)
}

func Bundle(userYamlPath, outputPath string) {

	yamlString, err := ioutil.ReadFile(userYamlPath)
	utils.CheckError(err, "Unable to read yaml Config.")

	config.ParseConfig(yamlString)
	client := runtime.NewClient(config.Config.TaskConfigs, config.Config.Options)

	fmt.Println(utils.Bold("Bundling " + userYamlPath + " to " + outputPath))

	client.Bundle(userYamlPath, outputPath)


}
