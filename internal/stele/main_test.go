package stele

import (
	"os"
	"testing"
)

// TestMain clears the GitHub Actions environment, so tests that do not set it
// themselves behave the same locally and in CI, and failing fixtures print no
// annotations into this repository's job log.
func TestMain(m *testing.M) {
	for _, name := range []string{"GITHUB_ACTIONS", "GITHUB_WORKSPACE"} {
		if err := os.Unsetenv(name); err != nil {
			panic(err)
		}
	}
	os.Exit(m.Run())
}
