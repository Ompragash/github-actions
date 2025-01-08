package utils

import (
	"fmt"
	"io/ioutil"
	"os"

	"strings"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
)

type workflow struct {
	Name string         `yaml:"name"`
	On   string         `yaml:"on"`
	Jobs map[string]job `yaml:"jobs"`
}

type job struct {
	Name   string `yaml:"name"`
	RunsOn string `yaml:"runs-on"`
	Steps  []step `yaml:"steps"`
}

type step struct {
	Id   string            `yaml:"id,omitempty"`
	Name string            `yaml:"name,omitempty"`
	Uses string            `yaml:"uses"`
	With map[string]string `yaml:"with"`
	Env  map[string]string `yaml:"env"`
	Run  string            `yaml:"run,omitempty"`
}

const (
	workflowEvent = "push"
	workflowName  = "drone-github-action"
	jobName       = "action"
	runsOnImage   = "ubuntu-latest"
)

func CreateWorkflowFile(ymlFile string, action string,
	with map[string]string, env map[string]string) error {

	// Main GH Action step with an id
	mainStep := step{
		Id:   "action_step",
		Uses: action,
		With: with,
		Env:  env,
	}

	// parse the GH Action's declared outputs
	clonePath := os.Getenv("DRONE_GITHUB_CLONE_PATH")
	outKeys, err := ParseActionOutputs(clonePath)
	if err != nil {
		fmt.Printf("warning: could not parse action.yml outputs from %s: %v\n", clonePath, err)
		outKeys = nil
	}

	// build a run script that echoes each discovered key to $DRONE_OUTPUT
	var sb strings.Builder
	for _, k := range outKeys {
		sb.WriteString(
			fmt.Sprintf("echo \"%s=${{ steps.action_step.outputs.%s }}\" >> $DRONE_OUTPUT\n", k, k),
		)
	}

	exportStep := step{
		Id:   "export_outputs",
		Name: "Export GH Action outputs to Drone",
		Run:  sb.String(),
	}

	jobSteps := []step{mainStep}
	if len(outKeys) > 0 {
		jobSteps = append(jobSteps, exportStep)
	}
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

	out, err := yaml.Marshal(&wf)
	if err != nil {
		return errors.Wrap(err, "failed to create action workflow yml")
	}

	if writeErr := ioutil.WriteFile(ymlFile, out, 0644); writeErr != nil {
		return errors.Wrap(err, "failed to write yml workflow file")
	}

	return nil
}

func getWorkflowEvent() string {
	buildEvent := os.Getenv("DRONE_BUILD_EVENT")
	if buildEvent == "push" || buildEvent == "pull_request" || buildEvent == "tag" {
		return buildEvent
	}
	return "custom"
}
