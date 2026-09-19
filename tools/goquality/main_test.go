package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunRejectsInvalidCommands(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name      string
		arguments []string
		message   string
	}{
		{name: "missing", message: "usage:"},
		{name: "unknown", arguments: []string{"unknown"}, message: "unknown quality check"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			var stderr bytes.Buffer
			if exitCode := run(test.arguments, &bytes.Buffer{}, &stderr); exitCode != 2 {
				t.Fatalf("exit code = %d, want 2", exitCode)
			}
			if !strings.Contains(stderr.String(), test.message) {
				t.Fatalf("stderr = %q, want text %q", stderr.String(), test.message)
			}
		})
	}
}

func TestParseCoverage(t *testing.T) {
	t.Parallel()

	profile := strings.NewReader("mode: atomic\ncore/example.go:1.1,2.2 3 1\ncore/example.go:4.1,5.2 2 0\n" +
		"other/tool.go:1.1,2.2 7 0\n")
	covered, total, err := parseCoverage(profile, "core/")
	if err != nil {
		t.Fatal(err)
	}
	if covered != 3 || total != 5 {
		t.Fatalf("coverage = %d/%d, want 3/5", covered, total)
	}
}

func TestParseCoverageRejectsInvalidProfiles(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name    string
		profile string
	}{
		{name: "missing header"},
		{name: "short record", profile: "mode: set\ninvalid\n"},
		{name: "invalid statements", profile: "mode: set\nexample.go:1.1,2.2 nope 1\n"},
		{name: "invalid executions", profile: "mode: set\nexample.go:1.1,2.2 1 nope\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			if _, _, err := parseCoverage(strings.NewReader(test.profile), ""); err == nil {
				t.Fatal("parseCoverage() error = nil, want an error")
			}
		})
	}
}
