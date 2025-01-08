// File: /utils/parse.go
package utils

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v2"
)

type GHActionSpec struct {
	Outputs map[string]interface{} `yaml:"outputs,omitempty"`
}

// ParseActionOutputs locates `action.yml` or `action.yaml` in `root` and
// returns all top-level outputs. e.g. ["aws-access-key-id","aws-secret-access-key"]
func ParseActionOutputs(root string) ([]string, error) {
	ymlPath := filepath.Join(root, "action.yml")
	yamlPath := filepath.Join(root, "action.yaml")

	var actionFile string
	switch {
	case fileExists(ymlPath):
		actionFile = ymlPath
	case fileExists(yamlPath):
		actionFile = yamlPath
	default:
		return nil, fmt.Errorf("no action.yml or action.yaml found in %s", root)
	}

	raw, err := ioutil.ReadFile(actionFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read action file: %w", err)
	}

	var spec GHActionSpec
	if err := yaml.Unmarshal(raw, &spec); err != nil {
		return nil, fmt.Errorf("failed to parse action.yml: %w", err)
	}

	keys := make([]string, 0, len(spec.Outputs))
	for k := range spec.Outputs {
		keys = append(keys, k)
	}
	return keys, nil
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}
