package utils

import (
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"
)

// ParseLookup parses a GitHub Action's `uses` string into repository URL and reference.
// Example: "actions/github-script@v5" -> "https://github.com/actions/github-script.git", "v5"
func ParseLookup(uses string) (repo string, ref string, ok bool) {
	parts := strings.Split(uses, "@")
	if len(parts) != 2 {
		logrus.Warnf("ParseLookup: invalid 'uses' format: %s", uses)
		return "", "", false
	}
	ownerRepo := parts[0]
	ref = parts[1]

	// Validate ownerRepo format
	if !strings.Contains(ownerRepo, "/") {
		logrus.Warnf("ParseLookup: invalid repository path: %s", ownerRepo)
		return "", "", false
	}

	// Convert to full GitHub repository URL
	repo = fmt.Sprintf("https://github.com/%s.git", ownerRepo)
	logrus.Infof("ParseLookup: Parsed 'uses' -> Repo: %s, Ref: %s", repo, ref)
	return repo, ref, true
}