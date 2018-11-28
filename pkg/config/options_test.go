package config

import (
	"testing"
)

func Test_Compile_WithOptions(t *testing.T) {
	runYaml := []byte(`
config:
  stop-on-failure: false
tasks:
  - name: Thing-a-ma-bob
    cmd: ./do/a/thing`)

	config, err := NewConfig(runYaml, nil)
	if err != nil {
		t.Errorf("expected no config error, got %+v", err)
	}

	// we are going to make certain that overridden options after compilation persist
	if config.Options.StopOnFailure != false {
		t.Errorf("expected stop-on-failure to be 'false', got '%v'", config.Options.StopOnFailure)
	}
}
