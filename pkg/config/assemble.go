package config

import (
	"regexp"
	"strings"

	"github.com/spf13/afero"
	"github.com/wagoodman/bashful/utils"
)

type assembler struct {
	filesystem afero.Fs
}

type includeMatch struct {
	includeFile string
	startIdx    int
	endIdx      int
}

func newAssembler(fs afero.Fs) *assembler {
	return &assembler{
		filesystem: fs,
	}
}

func getIndentSize(yamlString []byte, startIdx int) int {
	spaces := 0
	for idx := startIdx; idx > 0; idx++ {
		char := yamlString[idx]
		if char == '\n' {
			spaces = 0
		} else if char == ' ' {
			spaces++
		} else {
			break
		}
	}
	return spaces
}

func indentBytes(b []byte, size int) []byte {
	prefix := []byte(strings.Repeat(" ", size))
	var res []byte
	bol := true
	for _, c := range b {
		if bol && c != '\n' {
			res = append(res, prefix...)
		}
		res = append(res, c)
		bol = c == '\n'
	}
	return res
}

func (assembler *assembler) assemble(yamlString []byte) []byte {
	listInc := regexp.MustCompile(`(?m:\s*-\s\$include\s+(?P<filename>.+)$)`)
	mapInc := regexp.MustCompile(`(?m:^\s*\$include:\s+(?P<filename>.+)$)`)

	for _, pattern := range []*regexp.Regexp{listInc, mapInc} {
		for ok := true; ok; {
			indexes := pattern.FindSubmatchIndex(yamlString)
			ok = len(indexes) != 0
			if ok {
				match := includeMatch{
					includeFile: string(yamlString[indexes[2]:indexes[3]]),
					startIdx:    indexes[0],
					endIdx:      indexes[1],
				}

				indent := getIndentSize(yamlString, match.startIdx)

				contents, err := afero.ReadFile(assembler.filesystem, match.includeFile)
				utils.CheckError(err, "Unable to read file: "+match.includeFile)
				indentedContents := indentBytes(contents, indent)
				result := []byte{}
				result = append(result, yamlString[:match.startIdx]...)
				result = append(result, '\n')
				result = append(result, indentedContents...)
				result = append(result, yamlString[match.endIdx:]...)
				yamlString = result
			}
		}
	}

	return yamlString
}
