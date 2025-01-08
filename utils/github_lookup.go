package utils

import (
	"fmt"
	"strings"
)

// Parse GitHub Action's `uses` string into repository URL and reference.
// Example: "actions/github-script@v5" -> "https://github.com/actions/github-script.git", "v5"
func ParseLookup(uses string) (repo string, ref string, ok bool) {
	parts := strings.Split(uses, "@")
	if len(parts) != 2 {
		return "", "", false
	}
	ownerRepo := parts[0]
	ref = parts[1]

	// Convert to full GitHub repository URL
	repo = fmt.Sprintf("https://github.com/%s.git", ownerRepo)
	return repo, ref, true
}