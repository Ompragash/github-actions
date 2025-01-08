package plugin

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/drone-plugins/drone-github-actions/daemon"
	"github.com/drone-plugins/drone-github-actions/utils"
	"github.com/drone/plugin/cloner"
	"github.com/drone/plugin/plugin/github"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
)

const (
	envFile          = "/tmp/action.env"
	secretFile       = "/tmp/action.secrets"
	workflowFile     = "/tmp/workflow.yml"
	eventPayloadFile = "/tmp/event.json"
)

var (
	secrets = []string{"GITHUB_TOKEN"}
)

type (
	Action struct {
		Uses         string
		With         map[string]string
		Env          map[string]string
		Image        string
		EventPayload string // Webhook event payload
		Actor        string
		Verbose      bool
	}

	Plugin struct {
		Action Action
		Daemon daemon.Daemon // Docker daemon configuration
	}
)

type GHActionSpec struct {
    Outputs map[string]interface{} `yaml:"outputs,omitempty"`
}

// Exec executes the plugin step
func (p Plugin) Exec() error {
	if err := daemon.StartDaemon(p.Daemon); err != nil {
		return err
	}

	ctx := context.Background()
	repoURL, ref, ok := github.ParseLookup(p.Action.Uses)
	if !ok {
		logrus.Warnf("Invalid 'uses' format: %s", p.Action.Uses)
		return fmt.Errorf("invalid 'uses' format: %s", p.Action.Uses)
	}
	logrus.Infof("Parsed 'uses' string. Repo: %s, Ref: %s", repoURL, ref)

	// Clone the GH Action repository using `cloner` with parsed repo and ref
	clone := cloner.NewCache(cloner.NewDefault())
	codedir, cloneErr := clone.Clone(ctx, repoURL, ref, "")
	if cloneErr != nil {
		logrus.Warnf("Failed to clone GH Action: %v", cloneErr)
		codedir = "" // in case of cloning failure, proceed without local clone with empty value
	} else {
		logrus.Infof("Successfully cloned GH Action to %s", codedir)
	}

	outputFile := os.Getenv("DRONE_OUTPUT")

	// Get output variables from the cloned action
	outputVars, err := parseActionOutputs(codedir)
	if err != nil {
		logrus.Warnf("Could not parse action.yml outputs from %s: %v", codedir, err)
	}

	if err := utils.CreateWorkflowFile(workflowFile, p.Action.Uses,
		p.Action.With, p.Action.Env, outputFile, outputVars); err != nil {
		return err
	}

	if err := utils.CreateEnvAndSecretFile(envFile, secretFile, secrets); err != nil {
		return err
	}

	cmdArgs := []string{
		"-W",
		workflowFile,
		"-P",
		fmt.Sprintf("ubuntu-latest=%s", p.Action.Image),
		"--secret-file",
		secretFile,
		"--env-file",
		envFile,
		"-b",
		"--detect-event",
		"--container-options='-v $(pwd):$(pwd)'",
	}

	// optional arguments
	if p.Action.Actor != "" {
		cmdArgs = append(cmdArgs, "--actor")
		cmdArgs = append(cmdArgs, p.Action.Actor)
	}

	if p.Action.EventPayload != "" {
		if err := ioutil.WriteFile(eventPayloadFile, []byte(p.Action.EventPayload), 0644); err != nil {
			return errors.Wrap(err, "failed to write event payload to file")
		}

		cmdArgs = append(cmdArgs, "--eventpath", eventPayloadFile)
	}

	if p.Action.Verbose {
		cmdArgs = append(cmdArgs, "-v")
	}

	cmd := exec.Command("act", cmdArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	trace(cmd)

	err = cmd.Run()
	if err != nil {
		return err
	}
	return nil
}

// trace writes each command to stdout with the command wrapped in an xml
// tag so that it can be extracted and displayed in the logs.
func trace(cmd *exec.Cmd) {
	fmt.Fprintf(os.Stdout, "+ %s\n", strings.Join(cmd.Args, " "))
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

// fileExists is a helper function that checks if the path is an existing file.
func fileExists(path string) bool {
    info, err := os.Stat(path)
    if err != nil {
        return false
    }
    return !info.IsDir()
}