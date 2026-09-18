package stele

import (
	"errors"
	"go/build"
	"go/build/constraint"
	"os"
	"os/exec"
	"path/filepath"
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
	Output string `json:"Output"`
}

// planGoTest returns the batch of a Go test: its package, with the build tags
// its file requires. A test whose constraint does not hold on this platform
// did not run, without a process.
func planGoTest(root, path string) (testBatch, exactResult, bool) {
	absolute := filepath.Join(root, filepath.FromSlash(path))
	content, err := os.ReadFile(absolute)
	if err != nil {
		return testBatch{}, exactResult{err: err}, false
	}
	tags, satisfied := goBuildTags(content)
	if !satisfied {
		return testBatch{}, exactResult{}, false
	}
	moduleRoot := nearestGoModule(root, filepath.Dir(absolute))
	packagePath, err := relativePath(moduleRoot, filepath.Dir(absolute))
	if err != nil {
		return testBatch{}, exactResult{err: err}, false
	}
	return testBatch{
		runner:    "go",
		path:      filepath.ToSlash(filepath.Dir(filepath.FromSlash(path))),
		tags:      tags,
		directory: moduleRoot,
		target:    "./" + filepath.ToSlash(packagePath),
	}, exactResult{}, true
}

// goBatchCommand runs the selected top-level tests of one Go package in one
// process, without cached results and with the build tags of their files.
//
// @implements req.gosupport.d8c058038daa
func goBatchCommand(batch testBatch) *exec.Cmd {
	arguments := []string{"test", "-json", "-count=1", "-run", namePattern(batch.names())}
	if len(batch.tags) > 0 {
		arguments = append(arguments, "-tags="+strings.Join(batch.tags, ","))
	}
	arguments = append(arguments, batch.target)
	command := exec.Command("go", arguments...)
	command.Dir = batch.directory
	command.Env = append(os.Environ(), "STELE_CHILD_TEST=1")
	return command
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
