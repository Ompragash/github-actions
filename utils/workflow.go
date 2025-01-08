package utils

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
)

// GHActionSpec describes the optional `outputs` map in an action.yml
type GHActionSpec struct {
	Outputs map[string]interface{} `yaml:"outputs,omitempty"`
}

// workflow struct is the root of a GitHub Actions workflow file
type workflow struct {
	Name string         `yaml:"name"`
	On   string         `yaml:"on"`
	Jobs map[string]job `yaml:"jobs"`
}

// job represents a top-level job in a GitHub Actions workflow
type job struct {
	Name   string `yaml:"name"`
	RunsOn string `yaml:"runs-on"`
	Steps  []step `yaml:"steps"`
}

// step represents a single step in a GH Action job
type step struct {
	Id   string            `yaml:"id,omitempty"`
	Name string            `yaml:"name,omitempty"`
	Uses string            `yaml:"uses"`
	With map[string]string `yaml:"with"`
	Env  map[string]string `yaml:"env"`
	Run  string            `yaml:"run,omitempty"`
}

const (
	workflowName = "drone-github-action"
	jobName      = "action"
	runsOnImage  = "ubuntu-latest"
)

// CreateWorkflowFile builds an ephemeral GitHub Actions workflow file for `act`.
func CreateWorkflowFile(ymlFile string, action string, with map[string]string, env map[string]string) error {
	// This is the main GH Action step, including any 'with' and 'env' config
	mainStep := step{
		Id:   "action_step",
		Uses: action,
		With: with,
		Env:  env,
	}

	// Attempt to discover declared outputs from `action.yml` or `action.yaml`
	clonePath := os.Getenv("DRONE_GITHUB_CLONE_PATH")
	outKeys, err := parseActionOutputs(clonePath)
	if err != nil {
		fmt.Printf("warning: could not parse action.yml outputs from %s: %v\n", clonePath, err)
		outKeys = nil
	}

	// If we found any output keys, set up a command to echo them to DRONE_OUTPUT
	var sb strings.Builder
	for _, k := range outKeys {
		sb.WriteString(fmt.Sprintf("echo \"%s=${{ steps.action_step.outputs.%s }}\" >> $DRONE_OUTPUT\n", k, k))
	}

	// Step that appends GH Action outputs to the Drone environment file
	exportStep := step{
		Id:   "export_outputs",
		Name: "Export GH Action outputs to Drone",
		Run:  sb.String(),
	}

	// Build the array of steps in the job
	jobSteps := []step{mainStep}
	if len(outKeys) > 0 {
		jobSteps = append(jobSteps, exportStep)
	}

	// Construct the workflow with an event determined by getWorkflowEvent()
	wf := &workflow{
		Name: workflowName,
		On:   getWorkflowEvent(),
		Jobs: map[string]job{
			jobName: {
				Name:   jobName,
				RunsOn: runsOnImage,
				Steps:  jobSteps,
			},
		},
	}

	// Convert to YAML and write to file
	out, err := yaml.Marshal(&wf)
	if err != nil {
		return errors.Wrap(err, "failed to marshal action workflow to YAML")
	}

	if err := ioutil.WriteFile(ymlFile, out, 0644); err != nil {
		return errors.Wrap(err, "failed to write the workflow YAML file")
	}

	return nil
}

// parseActionOutputs locates and reads an `action.yml` or `action.yaml` to extract top-level `outputs`.
func parseActionOutputs(root string) ([]string, error) {
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

// getWorkflowEvent reads DRONE_BUILD_EVENT and maps push, pull_request, or tag to the same string.
// If it's something else, it defaults to "custom".
func getWorkflowEvent() string {
	buildEvent := os.Getenv("DRONE_BUILD_EVENT")
	if buildEvent == "push" || buildEvent == "pull_request" || buildEvent == "tag" {
		return buildEvent
	}
	return "custom"
}

// fileExists is a helper function that checks if the path is an existing file.
func fileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}
