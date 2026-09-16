package stele

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"go/build"
	"go/build/constraint"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"sort"
	"strings"
)

var errTestSkipped = errors.New("test was skipped")

var (
	goOperatingSystems = []string{
		"aix", "android", "darwin", "dragonfly", "freebsd", "hurd", "illumos", "ios", "js",
		"linux", "nacl", "netbsd", "openbsd", "plan9", "solaris", "wasip1", "windows", "zos",
	}
	goArchitectures = []string{
		"386", "amd64", "amd64p32", "arm", "arm64", "arm64be", "armbe", "loong64", "mips", "mips64",
		"mips64le", "mips64p32", "mips64p32le", "mipsle", "ppc", "ppc64", "ppc64le", "riscv", "riscv64",
		"s390", "s390x", "sparc", "sparc64", "wasm",
	}
	goUnixSystems = []string{
		"aix", "android", "darwin", "dragonfly", "freebsd", "hurd", "illumos", "ios",
		"linux", "netbsd", "openbsd", "solaris",
	}
)

type goTestEvent struct {
	Action string `json:"Action"`
	Test   string `json:"Test"`
}

// executeGoTest runs exactly one top-level Go test in its package, without
// cached results and with the build tags its file requires.
//
// @implements req.gosupport.d8c058038daa
func executeGoTest(root, path, selector string) (bool, bool, error) {
	absolute := filepath.Join(root, filepath.FromSlash(path))
	content, err := os.ReadFile(absolute)
	if err != nil {
		return false, false, err
	}
	tags, satisfied := goBuildTags(content)
	if !satisfied {
		return false, false, nil
	}
	moduleRoot := nearestGoModule(root, filepath.Dir(absolute))
	packagePath, err := relativePath(moduleRoot, filepath.Dir(absolute))
	if err != nil {
		return false, false, err
	}
	arguments := []string{"test", "-json", "-count=1", "-run", "^" + regexp.QuoteMeta(selector) + "$"}
	if len(tags) > 0 {
		arguments = append(arguments, "-tags="+strings.Join(tags, ","))
	}
	arguments = append(arguments, "./"+filepath.ToSlash(packagePath))
	command := exec.Command("go", arguments...)
	command.Dir = moduleRoot
	command.Env = append(os.Environ(), "STELE_CHILD_TEST=1")
	output, runErr := command.Output()
	switch goTestAction(output, selector) {
	case "pass":
		return runErr == nil, true, runErr
	case "skip":
		return false, true, errTestSkipped
	case "":
		return false, false, runErr
	default:
		return false, true, runErr
	}
}

// goTestAction returns the final pass, fail, or skip action reported for the
// top-level test, or "" when that test never ran.
func goTestAction(output []byte, selector string) string {
	action := ""
	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		var event goTestEvent
		if json.Unmarshal(scanner.Bytes(), &event) != nil || event.Test != selector {
			continue
		}
		switch event.Action {
		case "pass", "fail", "skip":
			action = event.Action
		}
	}
	return action
}

func nearestGoModule(root, directory string) string {
	current := directory
	for {
		if fileExists(filepath.Join(current, "go.mod")) {
			return current
		}
		parent := filepath.Dir(current)
		if current == root || parent == current {
			return root
		}
		current = parent
	}
}

// goBuildTags reads the file's //go:build constraint and returns the custom
// tags it requires, and whether the constraint then holds on this platform.
func goBuildTags(content []byte) ([]string, bool) {
	for line := range strings.SplitSeq(string(content), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "package ") {
			break
		}
		if !constraint.IsGoBuild(line) {
			continue
		}
		expression, err := constraint.Parse(line)
		if err != nil {
			return nil, false
		}
		tags := positiveCustomTags(expression)
		return tags, expression.Eval(func(tag string) bool {
			return slices.Contains(tags, tag) || platformTagHolds(tag)
		})
	}
	return nil, true
}

func positiveCustomTags(expression constraint.Expr) []string {
	found := make(map[string]bool)
	var visit func(constraint.Expr)
	visit = func(node constraint.Expr) {
		switch typed := node.(type) {
		case *constraint.TagExpr:
			if !isPlatformTag(typed.Tag) {
				found[typed.Tag] = true
			}
		case *constraint.AndExpr:
			visit(typed.X)
			visit(typed.Y)
		case *constraint.OrExpr:
			visit(typed.X)
			visit(typed.Y)
		}
	}
	visit(expression)
	tags := make([]string, 0, len(found))
	for tag := range found {
		tags = append(tags, tag)
	}
	sort.Strings(tags)
	return tags
}

func isPlatformTag(tag string) bool {
	switch tag {
	case "unix", "cgo", "gc", "gccgo":
		return true
	}
	return strings.HasPrefix(tag, "go1.") ||
		slices.Contains(goOperatingSystems, tag) ||
		slices.Contains(goArchitectures, tag)
}

func platformTagHolds(tag string) bool {
	switch tag {
	case runtime.GOOS, runtime.GOARCH, "gc":
		return true
	case "unix":
		return slices.Contains(goUnixSystems, runtime.GOOS)
	case "cgo":
		return build.Default.CgoEnabled
	}
	return slices.Contains(build.Default.ReleaseTags, tag)
}
