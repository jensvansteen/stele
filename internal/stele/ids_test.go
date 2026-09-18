package stele

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

const draftSpecPath = "openspec/changes/draft/specs/tasks/spec.md"

const draftSpec = `## ADDED Requirements

### Requirement: Add a task

The application SHALL add a task with non-empty text.

#### Scenario: Add a task with text

- **WHEN** the user submits text
- **THEN** a task is added

#### Scenario: Ignore blank text

- **WHEN** the user submits blank text
- **THEN** no task is added
`

var derivedIdentityLine = regexp.MustCompile(`(?m)^Verification-ID: ((?:req|scn)\.tasks\.[a-f0-9]{12})\r?$`)

func readTestFile(t *testing.T, root, relative string) string {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(relative)))
	if err != nil {
		t.Fatal(err)
	}
	return string(content)
}

func runIdentities(t *testing.T, arguments ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := Run(append([]string{"ids"}, arguments...), &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

// @verifies scn.ids.83b2e0efd623.unit
func TestIdsInsertsMissingIdentities(t *testing.T) {
	root := fixtureRoot(t)
	writeFixture(t, root, draftSpecPath, draftSpec)

	code, stdout, stderr := runIdentities(t, "--root", root, "--change", "draft")
	if code != 0 || !strings.Contains(stdout, "inserted 3 Verification-IDs") {
		t.Fatalf("stele ids = %d, %q, %q", code, stdout, stderr)
	}
	content := readTestFile(t, root, draftSpecPath)
	identities := derivedIdentityLine.FindAllStringSubmatch(content, -1)
	if len(identities) != 3 || !strings.HasPrefix(identities[0][1], "req.") {
		t.Fatalf("expected a requirement and two scenario IDs:\n%s", content)
	}
	lines := strings.Split(content, "\n")
	for _, heading := range []string{"### Requirement: Add a task", "#### Scenario: Ignore blank text"} {
		index := indexOfLine(lines, heading)
		if index < 0 || !strings.HasPrefix(lines[index+1], "Verification-ID: ") {
			t.Fatalf("no ID directly below %q:\n%s", heading, content)
		}
	}
	for _, identity := range identities {
		location := draftSpecPath + ":"
		if !strings.Contains(stdout, location) || !strings.Contains(stdout, identity[1]) {
			t.Fatalf("output does not report %s with its location: %q", identity[1], stdout)
		}
	}

	parsed, err := ParseSpecs(root, "draft")
	if err != nil {
		t.Fatal(err)
	}
	if len(parsed.Diagnostics) != 0 {
		t.Fatalf("proposal verification still reports problems: %#v", parsed.Diagnostics)
	}

	code, stdout, _ = runIdentities(t, "--root", root, "--change", "draft")
	if code != 0 || !strings.Contains(stdout, "every requirement and scenario has a Verification-ID") {
		t.Fatalf("second run = %d, %q", code, stdout)
	}
}

func indexOfLine(lines []string, want string) int {
	for index, line := range lines {
		if line == want {
			return index
		}
	}
	return -1
}

// @verifies scn.ids.1f44342aea82.unit
func TestIdsPreservesBytesAndExistingIdentities(t *testing.T) {
	root := fixtureRoot(t)
	original := strings.Join([]string{
		"# Tasks",
		"#### Scenario: Before any requirement",
		"### Requirement: Keep a task",
		"Verification-ID: req.custom.aaaaaaaaaaaa",
		"The application SHALL keep a task.",
		"#### Scenario: Invalid ID",
		"Verification-ID: not-an-id",
		"#### Scenario: Missing ID",
		"- **THEN** it gets one",
		"#### Scenario: Last heading",
	}, "\r\n")
	writeFixture(t, root, draftSpecPath, original)

	result, err := AssignIdentities(root, "draft", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Insertions) != 2 {
		t.Fatalf("expected two insertions, got %#v", result.Insertions)
	}
	for _, insertion := range result.Insertions {
		if !strings.HasPrefix(insertion.ID, "scn.custom.") {
			t.Fatalf("insertion does not reuse the file namespace: %#v", insertion)
		}
	}
	content := readTestFile(t, root, draftSpecPath)
	want := strings.Replace(original,
		"#### Scenario: Missing ID\r\n",
		"#### Scenario: Missing ID\r\nVerification-ID: "+result.Insertions[0].ID+"\r\n", 1)
	want += "\r\nVerification-ID: " + result.Insertions[1].ID
	if content != want {
		t.Fatalf("unexpected bytes:\n%q\nwant\n%q", content, want)
	}
	if result.Insertions[1].Line != 12 {
		t.Fatalf("written line = %d, want 12", result.Insertions[1].Line)
	}

	again, err := AssignIdentities(root, "draft", false)
	if err != nil || len(again.Insertions) != 0 || readTestFile(t, root, draftSpecPath) != content {
		t.Fatalf("second run changed the file: %#v, %v", again, err)
	}
}

// @verifies scn.ids.9211a14a8ec9.unit
func TestIdsCheckWritesNothing(t *testing.T) {
	root := fixtureRoot(t)
	writeFixture(t, root, draftSpecPath, draftSpec)

	code, stdout, stderr := runIdentities(t, "--root", root, "--change", "draft", "--check")
	if code != 1 || !strings.Contains(stdout, "3 headings lack a Verification-ID") {
		t.Fatalf("check = %d, %q, %q", code, stdout, stderr)
	}
	for _, title := range []string{"Add a task", "Add a task with text", "Ignore blank text"} {
		if !strings.Contains(stdout, `"`+title+`"`) {
			t.Fatalf("check does not list %q: %q", title, stdout)
		}
	}
	if !strings.Contains(stdout, draftSpecPath+":3 ") {
		t.Fatalf("check does not report the heading's own line: %q", stdout)
	}
	if readTestFile(t, root, draftSpecPath) != draftSpec {
		t.Fatal("check changed the spec")
	}

	if code, _, _ := runIdentities(t, "--root", root, "--change", "draft"); code != 0 {
		t.Fatalf("write = %d", code)
	}
	code, stdout, _ = runIdentities(t, "--root", root, "--change", "draft", "--check")
	if code != 0 || !strings.Contains(stdout, "every requirement and scenario") {
		t.Fatalf("check after writing = %d, %q", code, stdout)
	}
}

// @verifies scn.ids.0db6a666b9d4.unit
func TestIdsReportsJSON(t *testing.T) {
	outputs := make([]string, 0, 2)
	for range 2 {
		root := fixtureRoot(t)
		writeFixture(t, root, draftSpecPath, draftSpec)
		code, stdout, stderr := runIdentities(t, "--root", root, "--change", "draft", "--json")
		if code != 0 {
			t.Fatalf("json = %d, %q", code, stderr)
		}
		outputs = append(outputs, stdout)
	}
	if outputs[0] != outputs[1] {
		t.Fatalf("JSON is not deterministic:\n%s\n%s", outputs[0], outputs[1])
	}
	var result IdentityResult
	if err := json.Unmarshal([]byte(outputs[0]), &result); err != nil {
		t.Fatal(err)
	}
	if result.SchemaVersion != 1 || result.ChangeID != "draft" || result.Mode != "write" ||
		result.Verdict != "pass" || len(result.Insertions) != 3 {
		t.Fatalf("unexpected JSON result: %#v", result)
	}
	first := result.Insertions[0]
	if first.Kind != "requirement" || first.Title != "Add a task" || first.Path != draftSpecPath ||
		first.Line != 4 || first.Origin != "generated" {
		t.Fatalf("unexpected first insertion: %#v", first)
	}
	if second := result.Insertions[1]; second.Kind != "scenario" || second.Title != "Add a task with text" {
		t.Fatalf("unexpected second insertion: %#v", second)
	}
	for _, key := range []string{`"id"`, `"kind"`, `"title"`, `"path"`, `"line"`} {
		if !strings.Contains(outputs[0], key) {
			t.Fatalf("JSON lacks %s: %s", key, outputs[0])
		}
	}
}

// @verifies scn.ids.ed0d3d9619a5.unit
func TestIdsAreReproducible(t *testing.T) {
	contents := make([]string, 0, 2)
	for range 2 {
		root := fixtureRoot(t)
		writeFixture(t, root, draftSpecPath, draftSpec)
		if _, err := AssignIdentities(root, "draft", false); err != nil {
			t.Fatal(err)
		}
		contents = append(contents, readTestFile(t, root, draftSpecPath))
	}
	if contents[0] != contents[1] {
		t.Fatalf("copies received different IDs:\n%s\n%s", contents[0], contents[1])
	}

	// Repeated headings and other changes or capabilities derive different IDs.
	root := fixtureRoot(t)
	writeFixture(t, root, "openspec/changes/other/specs/tasks/spec.md", draftSpec)
	writeFixture(t, root, "openspec/changes/other/specs/platform/Auth-Tokens/spec.md",
		"### Requirement: Same\n#### Scenario: Same\n#### Scenario: Same\n")
	writeFixture(t, root, "openspec/changes/other/specs/!!!.md", "### Requirement: Loose file\n")
	result, err := AssignIdentities(root, "other", false)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	namespaces := map[string]bool{}
	for _, insertion := range result.Insertions {
		if seen[insertion.ID] {
			t.Fatalf("duplicate ID %s", insertion.ID)
		}
		seen[insertion.ID] = true
		namespaces[strings.Split(insertion.ID, ".")[1]] = true
		if strings.Contains(contents[0], insertion.ID) {
			t.Fatalf("another change derived the same ID %s", insertion.ID)
		}
	}
	for _, namespace := range []string{"tasks", "platformauthtokens", "spec"} {
		if !namespaces[namespace] {
			t.Fatalf("namespace %s missing from %#v", namespace, result.Insertions)
		}
	}
}

// @verifies scn.ids.d83366faeaf7.unit
func TestIdsAvoidDeclaredAndAnchoredIdentities(t *testing.T) {
	slot := &identitySlot{kind: requirementIdentityTarget, requirement: "Add a task"}
	first := deriveIdentity(slot, "draft", "tasks", "tasks", map[string]bool{})
	scenario := &identitySlot{kind: scenarioIdentityTarget, requirement: "Add a task", scenario: "Add a task with text"}
	firstScenario := deriveIdentity(scenario, "draft", "tasks", "tasks", map[string]bool{})
	blank := &identitySlot{kind: scenarioIdentityTarget, requirement: "Add a task", scenario: "Ignore blank text"}
	firstBlank := deriveIdentity(blank, "draft", "tasks", "tasks", map[string]bool{})

	root := fixtureRoot(t)
	writeFixture(t, root, draftSpecPath, draftSpec)
	writeFixture(t, root, "openspec/changes/archive/2026-01-01-old/specs/tasks/spec.md",
		"### Requirement: Old\nVerification-ID: "+first+"\n")
	writeFixture(t, root, "openspec/specs/tasks/spec.md",
		"### Requirement: Current\n#### Scenario: Current\nVerification-ID: "+firstScenario+"\n")
	writeFixture(t, root, "tests/tasks_test.go",
		"package tasks\n\n// @verifies "+firstBlank+".e2e\nfunc TestBlank() {}\n")

	result, err := AssignIdentities(root, "draft", false)
	if err != nil {
		t.Fatal(err)
	}
	for _, insertion := range result.Insertions {
		if insertion.ID == first || insertion.ID == firstScenario || insertion.ID == firstBlank {
			t.Fatalf("derived an existing ID: %#v", insertion)
		}
	}
	if len(result.Insertions) != 3 {
		t.Fatalf("unexpected insertions: %#v", result.Insertions)
	}
}

// @verifies scn.ids.11a49d3afd32.unit
func TestIdsReuseLivingSpecIdentities(t *testing.T) {
	root := fixtureRoot(t)
	writeFixture(t, root, "openspec/specs/tasks/spec.md", `### Requirement: Old title
Verification-ID: req.tasks.111111111111
#### Scenario: Kept scenario
Verification-ID: scn.tasks.222222222222
### Requirement: Old title
Verification-ID: req.tasks.999999999999
### Requirement: Invalid
Verification-ID: scn.tasks.333333333333
### Requirement: Already used
Verification-ID: req.tasks.444444444444
`)
	writeFixture(t, root, draftSpecPath, `## RENAMED Requirements
- FROM: `+"`### Requirement: Old title`"+`
- TO: `+"`### Requirement: New title`"+`

## MODIFIED Requirements
### Requirement: New title
#### Scenario: Kept scenario
#### Scenario: New scenario
### Requirement: Invalid
### Requirement: Already used
### Requirement: Also used
Verification-ID: req.tasks.444444444444

## ADDED Requirements
### Requirement: Kept scenario
`)
	result, err := AssignIdentities(root, "draft", false)
	if err != nil {
		t.Fatal(err)
	}
	origins := map[string]string{}
	identities := map[string]string{}
	for _, insertion := range result.Insertions {
		origins[insertion.Title] = insertion.Origin
		identities[insertion.Title] = insertion.ID
	}
	if identities["New title"] != "req.tasks.111111111111" || origins["New title"] != "base" {
		t.Fatalf("renamed requirement did not keep its ID: %#v", result.Insertions)
	}
	if origins["New scenario"] != "generated" {
		t.Fatalf("unexpected origins: %#v", result.Insertions)
	}
	content := readTestFile(t, root, draftSpecPath)
	if !strings.Contains(content, "#### Scenario: Kept scenario\nVerification-ID: scn.tasks.222222222222\n") {
		t.Fatalf("modified scenario did not keep its ID:\n%s", content)
	}
	if origins["Invalid"] != "generated" || origins["Already used"] != "generated" {
		t.Fatalf("invalid or already used IDs were reused: %#v", result.Insertions)
	}

	// Without a living spec, modified headings get new IDs.
	other := fixtureRoot(t)
	writeFixture(t, other, draftSpecPath, "## MODIFIED Requirements\n### Requirement: Unknown\n")
	unknown, err := AssignIdentities(other, "draft", false)
	if err != nil || len(unknown.Insertions) != 1 || unknown.Insertions[0].Origin != "generated" {
		t.Fatalf("missing living spec = %#v, %v", unknown, err)
	}
}

// @verifies scn.ids.03c9b0ba7536.unit
func TestIdsSkipRemovedAndRenamedSections(t *testing.T) {
	root := fixtureRoot(t)
	original := `## REMOVED Requirements
### Requirement: Retired
**Reason**: replaced

## RENAMED Requirements
- FROM: ### Requirement: Before
- TO: ### Requirement: After
### Requirement: Heading inside rename

## ADDED Requirements
### Requirement: Fresh
`
	writeFixture(t, root, draftSpecPath, original)
	result, err := AssignIdentities(root, "draft", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Insertions) != 1 || result.Insertions[0].Title != "Fresh" {
		t.Fatalf("expected only the added requirement, got %#v", result.Insertions)
	}
	content := readTestFile(t, root, draftSpecPath)
	untouched, _, _ := strings.Cut(original, "## ADDED")
	if !strings.HasPrefix(content, untouched) {
		t.Fatalf("removed or renamed sections changed:\n%s", content)
	}
}

func TestIdsReportInvocationAndFilesystemFailures(t *testing.T) {
	root := fixtureRoot(t)
	if code, _, stderr := runIdentities(t, "--root", root, "--change", "missing"); code != 2 ||
		!strings.Contains(stderr, "missing") {
		t.Fatalf("missing change = %d, %q", code, stderr)
	}
	if code, _, stderr := runIdentities(t, "--root", root); code != 2 ||
		!strings.Contains(stderr, "no OpenSpec change selected") {
		t.Fatalf("no change = %d, %q", code, stderr)
	}
	writeFixture(t, root, draftSpecPath, draftSpec)

	failure := errors.New("disk failure")
	cases := []struct {
		name  string
		setup func(t *testing.T)
	}{
		{"read", func(t *testing.T) {
			original := readSpecFile
			t.Cleanup(func() { readSpecFile = original })
			readSpecFile = func(string) ([]byte, error) { return nil, failure }
		}},
		{"read change", func(t *testing.T) {
			original := readSpecFile
			t.Cleanup(func() { readSpecFile = original })
			calls := 0
			readSpecFile = func(path string) ([]byte, error) {
				calls++
				if calls > 1 {
					return nil, failure
				}
				return os.ReadFile(path)
			}
		}},
		{"plan", func(t *testing.T) {
			original := readSpecFile
			t.Cleanup(func() { readSpecFile = original })
			calls := 0
			readSpecFile = func(path string) ([]byte, error) {
				calls++
				if calls > 2 {
					return nil, failure
				}
				return os.ReadFile(path)
			}
		}},
		{"anchors", func(t *testing.T) {
			original := scanIdentityAnchors
			t.Cleanup(func() { scanIdentityAnchors = original })
			scanIdentityAnchors = func(string) ([]Anchor, error) { return nil, failure }
		}},
		{"relative", func(t *testing.T) {
			original := relativePath
			t.Cleanup(func() { relativePath = original })
			relativePath = func(string, string) (string, error) { return "", failure }
		}},
		{"write", func(t *testing.T) {
			original := writeSpecFile
			t.Cleanup(func() { writeSpecFile = original })
			writeSpecFile = func(string, []byte, fs.FileMode) error { return failure }
		}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			testCase.setup(t)
			if _, err := AssignIdentities(root, "draft", false); !errors.Is(err, failure) {
				t.Fatalf("expected %v, got %v", failure, err)
			}
		})
	}
}

func TestIdsReportLivingSpecReadFailures(t *testing.T) {
	root := fixtureRoot(t)
	writeFixture(t, root, draftSpecPath, "## MODIFIED Requirements\n### Requirement: Changed\n")
	// A directory where the living spec should be cannot be read.
	if err := os.MkdirAll(filepath.Join(root, "openspec", "specs", "tasks", "spec.md"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := AssignIdentities(root, "draft", false); err == nil {
		t.Fatal("expected a living spec read error")
	}
}

func TestIdentityHelpers(t *testing.T) {
	if got := splitLinesKeepEnds(""); len(got) != 0 {
		t.Fatalf("splitLinesKeepEnds(\"\") = %#v", got)
	}
	if got := insertIdentityLines([]string{"### Requirement: Only"}, map[int]string{0: "req.x.aaaaaaaaaaaa"}); got !=
		"### Requirement: Only\nVerification-ID: req.x.aaaaaaaaaaaa" {
		t.Fatalf("insert without terminators = %q", got)
	}
	if lineTerminator("x") != "" || lineTerminator("x\r\n") != "\r\n" || lineTerminator("x\n") != "\n" {
		t.Fatal("lineTerminator returned an unexpected value")
	}
	if (openSpecBackend{}).Capability(changeScope("c"), "openspec/changes/c/specs/tasks.md") != "tasks" {
		t.Fatal("Capability did not strip a loose file's extension")
	}
	base := baseIdentities{}
	base.record("a", "req.x.aaaaaaaaaaaa", "req.")
	base.record("a", "req.x.bbbbbbbbbbbb", "req.")
	if base["a"] != "req.x.aaaaaaaaaaaa" {
		t.Fatalf("record overwrote the first identity: %#v", base)
	}
	var result IdentityResult
	renderIdentities(io.Discard, result)
}
