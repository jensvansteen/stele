package stele

import (
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"
)

// watched reports file changes as the client's watcher does.
func watched(client *lspTestClient, root string, relatives ...string) {
	changes := make([]map[string]any, 0, len(relatives))
	for _, relative := range relatives {
		changes = append(changes, map[string]any{"uri": fileURI(filepath.Join(root, relative)), "type": 2})
	}
	client.notify("workspace/didChangeWatchedFiles", map[string]any{"changes": changes})
}

// @verifies scn.languageserver.ba15e715dc8b.unit
func TestLSPReflectsAnUnsavedEdit(t *testing.T) {
	root := lspFixture(t)
	saved := "export const first = 1;\n\nexport function other() { return 2; }\n"
	writeFixture(t, root, "src/other.mts", saved)
	client := openLSP(t, root, lspCapabilities())
	uri := fileURI(filepath.Join(root, "src/other.mts"))
	client.notify("textDocument/didOpen", map[string]any{"textDocument": map[string]any{
		"uri": uri, "languageId": "typescript", "version": 1, "text": saved,
	}})
	client.notify("textDocument/didChange", map[string]any{
		"textDocument": map[string]any{"uri": uri, "version": 2},
		"contentChanges": []map[string]string{{"text": strings.Replace(saved, "\n\n",
			"\n\n// @implements "+"req.demo.aaaaaaaaaaaa\n", 1)}},
	})
	_, text := hoverAt(t, client, root, "src/other.mts", 2, 20)
	if !strings.Contains(text, "**Requirement:** Return value") {
		t.Fatalf("hover on the unsaved anchor = %q", text)
	}
	requireLocations(t, locationsAt(t, client, root, "textDocument/definition", position(root, lspChangeSpec, 3, 4)),
		"src/demo.mts:1:15-36", "src/other.mts:3:15-36")
	if content, _ := os.ReadFile(filepath.Join(root, "src/other.mts")); string(content) != saved {
		t.Fatal("the server wrote the unsaved edit")
	}
	client.notify("textDocument/didClose", map[string]any{"textDocument": map[string]any{"uri": uri}})
	requireLocations(t, locationsAt(t, client, root, "textDocument/definition", position(root, lspChangeSpec, 3, 4)),
		"src/demo.mts:1:15-36")
}

// @verifies scn.languageserver.4aa8ef7297c3.unit
func TestLSPPicksUpResultsFromATerminalRun(t *testing.T) {
	root := lspFixture(t)
	client := openLSP(t, root, lspCapabilities())
	if lenses := lensesOf(t, client, root, lspChangeSpec); !slices.Contains(lenses,
		"9: unit – not run · e2e – not run (stele.showStatus)") {
		t.Fatalf("lenses before the run = %q", lenses)
	}
	digest, err := ComputeInputDigest(root)
	if err != nil {
		t.Fatal(err)
	}
	lspStoredEvidence(t, root, "failed", digest)
	watched(client, root, defaultEvidencePath)
	if lenses := lensesOf(t, client, root, lspChangeSpec); !slices.Contains(lenses,
		"9: unit ✗ · e2e – not run (stele.showStatus)") {
		t.Fatalf("lenses after the run = %q", lenses)
	}
	diagnostics, _ := published(t, client, root, "tests/value.test.mts")
	if found := withCode(diagnostics, "EXECUTION_FAILED"); len(found) != 1 || found[0].Range.Start.Line != 1 {
		t.Fatalf("diagnostics after the run = %#v", diagnostics)
	}
	if refresh := client.take("workspace/codeLens/refresh"); len(refresh) != 1 {
		t.Fatalf("CodeLens refresh requests = %#v", refresh)
	}
}

// @verifies scn.languageserver.39afeb90a2e9.unit
func TestLSPPicksUpANewSpecificationFile(t *testing.T) {
	root := lspFixture(t)
	writeEvidenceTest(t, root, "tests/todo.test.mts", "scn.todo.222222222222", "adds a todo")
	client := openLSP(t, root, lspCapabilities())
	if diagnostics, _ := published(t, client, root, "tests/todo.test.mts"); len(withCode(diagnostics,
		"ANCHOR_DANGLING")) != 1 {
		t.Fatalf("before the new spec = %#v", diagnostics)
	}
	writeFixture(t, root, "openspec/changes/todo/specs/todo/spec.md", `<!-- stele: spec v1 -->
## ADDED Requirements

### Requirement: Add todos
Verification-ID: req.todo.111111111111

The system SHALL add todos.

#### Scenario: Add a todo
Verification-ID: scn.todo.222222222222

- **WHEN** the user adds a todo
- **THEN** it is listed
`)
	client.notify("workspace/didChangeWatchedFiles", map[string]any{"changes": []map[string]any{
		{"uri": fileURI(filepath.Join(root, "openspec/changes/todo")), "type": 1},
	}})
	if diagnostics, found := published(t, client, root, "tests/todo.test.mts"); !found || len(diagnostics) != 0 {
		t.Fatalf("after the new spec = %#v", diagnostics)
	}
	lenses := withCommand(lensesOf(t, client, root, "openspec/changes/todo/specs/todo/spec.md"), "stele.showStatus")
	if !slices.Equal(lenses, []string{
		"4: not run · 1 not run (stele.showStatus)", "9: tests – not run (stele.showStatus)",
	}) {
		t.Fatalf("lenses of the new spec = %q", lenses)
	}
	if err := os.RemoveAll(filepath.Join(root, "openspec/changes/todo")); err != nil {
		t.Fatal(err)
	}
	watched(client, root, "openspec/changes/todo")
	if lenses := lensesOf(t, client, root, "openspec/changes/todo/specs/todo/spec.md"); len(lenses) != 0 {
		t.Fatalf("lenses of a deleted spec = %q", lenses)
	}
}

// tickingTimers returns a timer source whose poll timers fire when the test
// sends on the returned channel; debounce timers never fire.
func tickingTimers() (func(time.Duration) <-chan time.Time, chan time.Time) {
	ticks := make(chan time.Time)
	return func(duration time.Duration) <-chan time.Time {
		if duration == lspPollInterval {
			return ticks
		}
		return nil
	}, ticks
}

// @verifies scn.languageserver.a8fc0b7b6625.unit
func TestLSPWatchesFilesWithoutClientSupport(t *testing.T) {
	root := lspFixture(t)
	writeFixture(t, root, "tests/e2e/value.test.mts", "export const nothing = 1;\n")
	after, ticks := tickingTimers()
	client := startLSP(t, after)
	client.initializeLSP(root, map[string]any{})
	requireLocations(t, locationsAt(t, client, root, "textDocument/definition", position(root, lspChangeSpec, 8, 4)),
		"tests/value.test.mts:2:13-39")
	if registrations := client.take("client/registerCapability"); len(registrations) != 0 {
		t.Fatalf("registered watchers without client support: %#v", registrations)
	}
	writeEvidenceTest(t, root, "tests/e2e/value.test.mts", evidenceScenarioID+".e2e", "shows the value")
	ticks <- time.Time{}
	requireLocations(t, locationsAt(t, client, root, "textDocument/definition", position(root, lspChangeSpec, 8, 4)),
		"tests/e2e/value.test.mts:2:13-38", "tests/value.test.mts:2:13-39")
	ticks <- time.Time{}
	client.sync()
	registered := openLSP(t, root, lspCapabilities())
	registered.sync()
	if registrations := registered.take("client/registerCapability"); len(registrations) != 1 ||
		!strings.Contains(string(registrations[0].Params), "workspace/didChangeWatchedFiles") {
		t.Fatalf("watcher registration = %#v", registrations)
	}
}

// lspDebounceTimers returns a timer source whose debounce timers fire when
// the test sends on the returned channel.
func lspDebounceTimers() (func(time.Duration) <-chan time.Time, chan time.Time) {
	fire := make(chan time.Time)
	return func(duration time.Duration) <-chan time.Time {
		if duration == lspDebounce {
			return fire
		}
		return nil
	}, fire
}

func TestLSPRebuildsAfterTheDebounce(t *testing.T) {
	root := lspFixture(t)
	after, fire := lspDebounceTimers()
	client := startLSP(t, after)
	client.initializeLSP(root, lspCapabilities())
	uri := fileURI(filepath.Join(root, "src/demo.mts"))
	client.notify("textDocument/didOpen", map[string]any{"textDocument": map[string]any{
		"uri": uri, "languageId": "typescript", "version": 1, "text": "// @implements req.demo.000000000000\n",
	}})
	fire <- time.Time{}
	client.notify("textDocument/didSave", map[string]any{"textDocument": map[string]any{"uri": uri}})
	client.notify("textDocument/didOpen", map[string]any{"textDocument": map[string]any{"uri": "untitled:1"}})
	client.notify("textDocument/didOpen", map[string]any{"textDocument": map[string]any{
		"uri": fileURI(filepath.Join(t.TempDir(), "x.md")),
	}})
	client.notify("textDocument/didChange", map[string]any{"textDocument": map[string]any{
		"uri": fileURI(filepath.Join(root, "src/closed.mts")),
	}})
	client.notify("workspace/didChangeWatchedFiles", map[string]any{"changes": []map[string]any{
		{"uri": "untitled:1"}, {"uri": fileURI(filepath.Join(t.TempDir(), "y.md"))},
	}})
	client.sync()
	if diagnostics := client.take("textDocument/publishDiagnostics"); len(diagnostics) != 2 ||
		!strings.Contains(string(diagnostics[1].Params), "ANCHOR_DANGLING") {
		t.Fatalf("debounced diagnostics = %#v", diagnostics)
	}
}

// @verifies scn.languageserver.82c1741b92aa.unit
func TestLSPIncrementalUpdatesMatchAFullRebuild(t *testing.T) {
	for seed := range uint64(6) {
		t.Run(fmt.Sprintf("seed %d", seed), func(t *testing.T) {
			root := lspFixture(t)
			files := newLSPFiles(root)
			if _, err := buildLSPProject(files); err != nil {
				t.Fatal(err)
			}
			random := rand.New(rand.NewPCG(seed, 7))
			for step := range 12 {
				relative := applyRandomEdit(t, root, random, step)
				files.update(relative)
				if random.IntN(3) == 0 {
					incremental, err := buildLSPProject(files)
					if err != nil {
						t.Fatal(err)
					}
					requireSameBuild(t, incremental, root)
				}
			}
			incremental, err := buildLSPProject(files)
			if err != nil {
				t.Fatal(err)
			}
			requireSameBuild(t, incremental, root)
		})
	}
}

// applyRandomEdit changes, creates, or deletes one input file and returns its
// repository path.
func applyRandomEdit(t *testing.T, root string, random *rand.Rand, step int) string {
	t.Helper()
	candidates := []string{
		"src/demo.mts", "tests/value.test.mts", "tests/extra.test.mts", lspChangeSpec,
		"openspec/changes/example/linkage-plan.json", "openspec/specs/demo/spec.md", defaultEvidencePath,
		"openspec/changes/second/specs/demo/spec.md",
	}
	relative := candidates[random.IntN(len(candidates))]
	path := filepath.Join(root, filepath.FromSlash(relative))
	switch random.IntN(4) {
	case 0:
		_ = os.Remove(path)
	case 1:
		writeFixture(t, root, relative, lspRandomContent(relative, step))
	default:
		content, _ := os.ReadFile(path)
		writeFixture(t, root, relative, string(content)+lspRandomContent(relative, step))
	}
	return relative
}

// lspRandomContent is content that changes what the file contributes.
func lspRandomContent(relative string, step int) string {
	switch {
	case strings.HasSuffix(relative, ".md"):
		return fmt.Sprintf("\n### Requirement: Step %d\nVerification-ID: req.demo.%012d\n\nText %d.\n",
			step, step, step)
	case strings.HasSuffix(relative, ".json"):
		return fmt.Sprintf(`{"schemaVersion":2,"changeId":"example",`+
			`"scenarios":{"scn.demo.%012d":{"evidence":[]}}}`, step)
	default:
		return fmt.Sprintf("\n// @implements "+"req.demo.%012d\nexport function step%d() {}\n", step, step)
	}
}

func requireSameBuild(t *testing.T, incremental *lspBuild, root string) {
	t.Helper()
	full, err := buildLSPProject(newLSPFiles(root))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(incremental.index, full.index) || !reflect.DeepEqual(incremental.findings, full.findings) {
		t.Fatalf("incremental build differs from a full rebuild:\n%#v\n%#v", incremental.findings, full.findings)
	}
}

func TestLSPFiles(t *testing.T) {
	root := lspFixture(t)
	writeFixture(t, root, "node_modules/x/spec.md", "x")
	writeFixture(t, root, "docs/guide.md", "x")
	files := newLSPFiles(root)
	if !files.view(false).isFile(filepath.Join(root, "src/demo.mts")) ||
		files.view(false).isFile(filepath.Join(root, "docs/guide.md")) {
		t.Fatalf("tracked files = %v", files.keys())
	}
	if trackedPath("node_modules/x/spec.md") || !trackedPath(defaultEvidencePath) || trackedPath("tests/a.txt") ||
		!trackedPath("go.mod") {
		t.Fatal("tracked paths")
	}
	if files.refresh() {
		t.Fatal("an unchanged tree changed")
	}
	writeFixture(t, root, "src/demo.mts", "// changed and longer\n")
	if !files.refresh() {
		t.Fatal("a changed file was missed")
	}
	if _, err := files.view(false).readFile(filepath.Join(root, "src/missing.mts")); err == nil {
		t.Fatal("a missing file was read")
	}
	if err := os.Remove(filepath.Join(root, "src/demo.mts")); err != nil {
		t.Fatal(err)
	}
	if _, err := files.view(false).readFile(filepath.Join(root, "src/demo.mts")); err == nil {
		t.Fatal("a file deleted without an update was read")
	}
	if paths := files.view(false).walk("/elsewhere", func(string) bool { return true }); len(paths) != 0 {
		t.Fatalf("walk outside the root = %v", paths)
	}
	files.overlays["src/open.mts"] = []byte("x")
	names, directories := files.view(true).children(filepath.Join(root, "src"))
	if !slices.Equal(names, []string{"demo.mts", "open.mts"}) || len(directories) != 0 {
		t.Fatalf("children = %v %v", names, directories)
	}
	if _, directories := files.view(true).children(root); !slices.Contains(directories, "openspec") {
		t.Fatalf("root directories = %v", directories)
	}
	if relative, inside := files.relative(root); !inside || relative != "" {
		t.Fatal("the root is its own empty path")
	}
}
