package utils

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
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
    Uses string            `yaml:"uses,omitempty"`
    With map[string]string `yaml:"with,omitempty"`
    Env  map[string]string `yaml:"env,omitempty"`
    Run  string            `yaml:"run,omitempty"`
}

const (
    workflowName = "drone-github-action"
    jobName      = "action"
    runsOnImage  = "ubuntu-latest"
)

// CreateWorkflowFile generates the ephemeral GitHub Actions workflow file for `act`.
// It parses `action.yml` or `action.yaml` to extract outputs and injects an export step if outputs exist.
func CreateWorkflowFile(ymlFile string, action string, with map[string]string, env map[string]string) error {
    logrus.Infof("Creating workflow file at %s with action: %s", ymlFile, action)

    // Initialize the main GitHub Action step
    mainStep := step{
        Id:   "action_step",
        Uses: action,
        With: with,
        Env:  env,
    }

    // Initialize the job with the main step
    j := job{
        Name:   jobName,
        RunsOn: runsOnImage,
        Steps:  []step{mainStep},
    }

    // Create the workflow structure
    wf := &workflow{
        Name: workflowName,
        On:   getWorkflowEvent(),
        Jobs: map[string]job{
            jobName: j,
        },
    }

    // Retrieve the DRONE_OUTPUT path from environment variable
    droneOutputPath := os.Getenv("DRONE_OUTPUT")
    if droneOutputPath == "" {
        logrus.Warn("DRONE_OUTPUT is not set; skipping output export step")
    } else {
        // Attempt to parse outputs from action.yml/yaml
        clonePath := os.Getenv("DRONE_GITHUB_CLONE_PATH")
        if clonePath == "" {
            logrus.Warn("DRONE_GITHUB_CLONE_PATH is empty; skipping action.yml parsing for outputs")
        } else {
            outKeys, err := parseActionOutputs(clonePath)
            if err != nil {
                logrus.Warnf("Could not parse action.yml outputs from %s: %v", clonePath, err)
            }

            if len(outKeys) > 0 {
                logrus.Infof("Found outputs in action.yml: %v", outKeys)
                // Ensure the output directory exists
                outputDir := filepath.Dir(droneOutputPath)
                if err := os.MkdirAll(outputDir, 0755); err != nil {
                    logrus.Errorf("Failed to create output directory %s: %v", outputDir, err)
                    return errors.Wrap(err, "failed to create output directory")
                }

                // Build a run script that echoes each discovered key to $DRONE_OUTPUT
                var sb strings.Builder
                sb.WriteString("#!/bin/bash\nset -e\n")
                for _, k := range outKeys {
                    // Escape double quotes and dollar signs to prevent shell injection
                    keyEscaped := escapeShell(k)
                    sb.WriteString(fmt.Sprintf("echo \"%s=${{ steps.action_step.outputs.%s }}\" >> %s\n", keyEscaped, keyEscaped, droneOutputPath))
                }

                // Define the export step
                exportStep := step{
                    Id:   "export_outputs",
                    Name: "Export GH Action outputs to Drone",
                    Run:  sb.String(),
                }

                // Retrieve the existing job from the map
                existingJob, exists := wf.Jobs[jobName]
                if !exists {
                    logrus.Errorf("Job %s does not exist in workflow", jobName)
                    return errors.Errorf("job %s does not exist in workflow", jobName)
                }

                // Append the export step to the existing job's steps
                existingJob.Steps = append(existingJob.Steps, exportStep)

                // Set the modified job back into the workflow's Jobs map
                wf.Jobs[jobName] = existingJob
            } else {
                logrus.Infof("No outputs found in action.yml; skipping export step")
            }
        }
    }

    // Marshal the workflow struct to YAML
    out, err := yaml.Marshal(&wf)
    if err != nil {
        logrus.Errorf("Failed to marshal workflow to YAML: %v", err)
        return errors.Wrap(err, "failed to marshal workflow to YAML")
    }

    // **Print the generated workflow YAML for debugging**
    logrus.Infof("Generated workflow.yml:\n%s", string(out))

    // Write the YAML to the specified file
    if err := ioutil.WriteFile(ymlFile, out, 0644); err != nil {
        logrus.Errorf("Failed to write workflow YAML file: %v", err)
        return errors.Wrap(err, "failed to write workflow YAML file")
    }

    logrus.Infof("Successfully wrote workflow YAML file to %s", ymlFile)
    return nil
}

// parseActionOutputs locates `action.yml` or `action.yaml` in `root` and returns all top-level outputs.
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

// escapeShell escapes double quotes and dollar signs in a string for safe shell usage.
func escapeShell(s string) string {
    s = strings.ReplaceAll(s, "\"", "\\\"")
    s = strings.ReplaceAll(s, "$", "\\$")
    return s
}
