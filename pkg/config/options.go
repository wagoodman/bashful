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

package config

// NewOptions creates a new Options populated with sane default values
func NewOptions() *Options {
	return &Options{
		BulletChar:           "•",
		CollapseOnCompletion: false,
		ColorError:           160,
		ColorPending:         22,
		ColorRunning:         22,
		ColorSuccess:         10,
		EventDriven:          true,
		ExecReplaceString:    "<exec>",
		IgnoreFailure:        false,
		MaxParallelCmds:      4,
		ReplicaReplaceString: "<replace>",
		ShowFailureReport:    true,
		ShowSummaryErrors:    false,
		ShowSummaryFooter:    true,
		ShowSummarySteps:     true,
		ShowSummaryTimes:     true,
		ShowTaskEta:          false,
		ShowTaskOutput:       true,
		StopOnFailure:        true,
		SingleLineDisplay:    false,
		UpdateInterval:       -1,
	}
}

// UnmarshalYAML parses and creates a Options from a given user yaml string
func (options *Options) UnmarshalYAML(unmarshal func(interface{}) error) error {
	type defaults Options
	defaultValues := defaults(*NewOptions())

	if err := unmarshal(&defaultValues); err != nil {
		return err
	}

	*options = Options(defaultValues)

	if options.SingleLineDisplay {
		options.ShowSummaryFooter = false
		options.CollapseOnCompletion = false
	}

	// the global options must be available when parsing the task yaml (todo: does order matter?)
	globalOptions = options
	return nil
}
