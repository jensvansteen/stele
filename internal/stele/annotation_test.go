package stele

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/fs"
	"strings"
	"testing"
)

const annotatedSpecBody = `## ADDED Requirements

### Requirement: Return value
Verification-ID: req.demo.aaaaaaaaaaaa

The system SHALL return the stored value.

#### Scenario: Value is returned
Verification-ID: scn.demo.bbbbbbbbbbbb

- **WHEN** a value is stored
- **THEN** it is returned
`

const exampleSpecPath = "openspec/changes/example/specs/demo/spec.md"

// parseExample writes one delta spec with the given first lines and parses it.
func parseExample(t *testing.T, head string) ParsedSpecs {
	t.Helper()
	root := fixtureRoot(t)
	writeFixture(t, root, exampleSpecPath, head+annotatedSpecBody)
	parsed, err := ParseSpecs(root, "example")
	if err != nil {
		t.Fatal(err)
	}
	if len(parsed.Requirements) != 1 || parsed.Requirements[0].ID != "req.demo.aaaaaaaaaaaa" ||
		len(parsed.Requirements[0].Scenarios) != 1 {
		t.Fatalf("the annotation changed how the file is read: %#v", parsed.Requirements)
	}
	return parsed
}

func annotationDiagnostics(parsed ParsedSpecs) []Diagnostic {
	found := make([]Diagnostic, 0)
	for _, item := range parsed.Diagnostics {
		if strings.HasPrefix(item.Code, "SPEC_ANNOTATION_") {
			found = append(found, item)
		}
	}
	return found
}

func onlyAnnotation(t *testing.T, parsed ParsedSpecs) SpecAnnotation {
	t.Helper()
	if len(parsed.Annotations) != 1 || parsed.Annotations[0].Path != exampleSpecPath {
		t.Fatalf("annotations = %#v", parsed.Annotations)
	}
	return parsed.Annotations[0]
}

// @verifies scn.specannotation.0de8bd2cbe54.unit
func TestRecognizesTheCanonicalAnnotation(t *testing.T) {
	parsed := parseExample(t, "<!-- stele: spec v1 -->\n")
	if annotation := onlyAnnotation(t, parsed); annotation.State != annotationAnnotated || annotation.Version != "v1" {
		t.Fatalf("canonical annotation = %#v", annotation)
	}
	if found := annotationDiagnostics(parsed); len(found) != 0 {
		t.Fatalf("canonical annotation reports %#v", found)
	}
}

// @verifies scn.specannotation.0d7c07caa518.unit
func TestAnnotationToleratesWhitespaceLineEndingsAndByteOrderMark(t *testing.T) {
	head := byteOrderMark + "  <!--stele:   spec\tv1-->\r\n"
	parsed := parseExample(t, head)
	if annotation := onlyAnnotation(t, parsed); annotation.State != annotationAnnotated || annotation.Version != "v1" {
		t.Fatalf("tolerated annotation = %#v", annotation)
	}
	if found := annotationDiagnostics(parsed); len(found) != 0 {
		t.Fatalf("tolerated annotation reports %#v", found)
	}
	for _, line := range []string{
		"<!-- stele: spec v1 -->",
		"\t<!--\tstele:\tspec  v1\t-->\t",
		"<!--stele:spec v1-->",
	} {
		if result := classifyAnnotation("spec.md", line+"\r\n# Title").result(); result.State != annotationAnnotated {
			t.Fatalf("%q = %#v", line, result)
		}
	}
}

// @verifies scn.specannotation.6434ff2d493c.unit
func TestAnnotationFieldsAreIgnoredWithAWarning(t *testing.T) {
	plain := parseExample(t, "<!-- stele: spec v1 -->\n")
	parsed := parseExample(t, "<!-- stele: spec v1; targets: vscode, zed; owner -->\n")
	if annotation := onlyAnnotation(t, parsed); annotation.State != annotationAnnotated || annotation.Version != "v1" {
		t.Fatalf("annotation with fields = %#v", annotation)
	}
	found := annotationDiagnostics(parsed)
	if len(found) != 2 {
		t.Fatalf("expected one warning per field, got %#v", found)
	}
	for index, field := range []string{`"owner"`, `"targets"`} {
		if found[index].Code != "SPEC_ANNOTATION_FIELD_IGNORED" || found[index].Severity != "warning" ||
			!strings.Contains(found[index].Message, field) {
			t.Fatalf("warning %d = %#v", index, found[index])
		}
	}
	if errors, _, verdict := diagnosticSummary(parsed.Diagnostics); errors != 0 || verdict != "pass" ||
		len(parsed.Requirements) != len(plain.Requirements) ||
		parsed.Requirements[0].Text != plain.Requirements[0].Text {
		t.Fatalf("fields changed verification: %#v", parsed)
	}

	for line, want := range map[string]string{
		"<!-- stele: spec v1; a: b -- c -->":         `"a: b -- c"`,
		"<!-- stele: spec v1; ; Owner: me -->":       `""`,
		"<!-- stele: spec v1;targets:x;targets:y-->": `"targets"`,
	} {
		classifier := classifyAnnotation("spec.md", line)
		if classifier.result().State != annotationAnnotated || len(classifier.fields) == 0 ||
			!strings.Contains(classifier.diagnostics("fix")[0].Message, want) {
			t.Fatalf("%q = %#v, %#v", line, classifier.result(), classifier.fields)
		}
	}
}

// @verifies scn.specannotation.c1a89d2bc235.unit
func TestUnsupportedAnnotationVersionFails(t *testing.T) {
	parsed := parseExample(t, "<!-- stele: spec v2 -->\n")
	annotation := onlyAnnotation(t, parsed)
	if annotation.State != annotationUnsupported || annotation.Version != "v2" {
		t.Fatalf("unsupported annotation = %#v", annotation)
	}
	found := annotationDiagnostics(parsed)
	if len(found) != 1 || found[0].Code != "SPEC_ANNOTATION_UNSUPPORTED" || found[0].Severity != "error" ||
		!strings.Contains(found[0].Message, exampleSpecPath) || !strings.Contains(found[0].Message, "v2") {
		t.Fatalf("unsupported diagnostics = %#v", found)
	}
	if _, _, verdict := diagnosticSummary(parsed.Diagnostics); verdict != "fail" {
		t.Fatal("an unsupported version did not fail verification")
	}
}

// @verifies scn.specannotation.10178af5c550.unit
func TestMalformedAnnotationFails(t *testing.T) {
	for _, line := range []string{
		"<!-- stele: spec -->",
		"<!-- stele: specification v1 -->",
		"<!-- stele: spec v1",
		"<!-- stele: spec v1 --> trailing",
		"<!-- stele: spec v1x -->",
		"<!-- stele: Spec v1 -->",
	} {
		parsed := parseExample(t, line+"\n")
		annotation := onlyAnnotation(t, parsed)
		if annotation.State != annotationMalformed || annotation.Version != "" {
			t.Fatalf("%q = %#v", line, annotation)
		}
		found := annotationDiagnostics(parsed)
		if len(found) != 1 || found[0].Code != "SPEC_ANNOTATION_MALFORMED" || found[0].Severity != "error" ||
			!strings.Contains(found[0].Message, exampleSpecPath) {
			t.Fatalf("%q diagnostics = %#v", line, found)
		}
		if _, _, verdict := diagnosticSummary(parsed.Diagnostics); verdict != "fail" {
			t.Fatalf("%q did not fail verification", line)
		}
	}
	// Other first-line comments, including a capitalized marker, are not annotations.
	for _, line := range []string{"<!-- a note -->", "<!-- Stele: spec v1 -->", "# Title"} {
		if state := classifyAnnotation("spec.md", line).result().State; state != annotationMissing {
			t.Fatalf("%q = %s", line, state)
		}
	}
}

// @verifies scn.specannotation.2681b7fc2880.unit
func TestAnnotationBelowTheFirstLineIsMisplaced(t *testing.T) {
	root := fixtureRoot(t)
	writeFixture(t, root, exampleSpecPath, "# Demo\n\n<!-- stele: spec v1 -->\n"+annotatedSpecBody)
	policies := map[string]string{"": "warning", unannotatedWarn: "warning", unannotatedError: "error"}
	for policy, severity := range policies {
		parsed, err := parseScopeSpecs(root, verificationScope{changeID: "example", unannotated: policy})
		if err != nil {
			t.Fatal(err)
		}
		if annotation := onlyAnnotation(t, parsed); annotation.State != annotationMisplaced {
			t.Fatalf("misplaced annotation = %#v", annotation)
		}
		found := annotationDiagnostics(parsed)
		if len(found) != 1 || found[0].Code != "SPEC_ANNOTATION_MISPLACED" || found[0].Source.Line != 3 ||
			found[0].Severity != severity || !strings.Contains(found[0].Message, "line 3") {
			t.Fatalf("policy %q diagnostics = %#v", policy, found)
		}
	}

	// A later annotation line is not misplaced when the first line is valid.
	if state := classifyAnnotation("spec.md", annotationCanonical+"\n"+annotationCanonical).result().State; state !=
		annotationAnnotated {
		t.Fatalf("duplicate annotation = %s", state)
	}
}

func runAnnotate(t *testing.T, arguments ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := Run(append([]string{"annotate"}, arguments...), &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

// @verifies scn.specannotation.c1e6a83408c6.unit
func TestAnnotatePreservesEveryOtherByte(t *testing.T) {
	root := fixtureRoot(t)
	crlf := "## ADDED Requirements\r\n\r\n### Requirement: Keep\r\n" +
		"The system SHALL keep bytes.\r\n\r\n#### Scenario: Kept"
	annotated := annotationCanonical + "\n## ADDED Requirements\n"
	writeFixture(t, root, "openspec/changes/example/specs/crlf/spec.md", crlf)
	writeFixture(t, root, "openspec/changes/example/specs/done/spec.md", annotated)

	code, stdout, stderr := runAnnotate(t, "--root", root, "--change", "example")
	if code != 0 || !strings.Contains(stdout, "annotated 1 specification files") ||
		!strings.Contains(stdout, "annotated openspec/changes/example/specs/crlf/spec.md") ||
		strings.Contains(stdout, "specs/done/spec.md") {
		t.Fatalf("annotate = %d, %q, %q", code, stdout, stderr)
	}
	if content := readTestFile(t, root, "openspec/changes/example/specs/crlf/spec.md"); content !=
		annotationCanonical+"\r\n"+crlf {
		t.Fatalf("unexpected bytes: %q", content)
	}
	if readTestFile(t, root, "openspec/changes/example/specs/done/spec.md") != annotated {
		t.Fatal("an annotated file changed")
	}

	for content, want := range map[string]string{
		"":                           annotationCanonical + "\n",
		"# Title":                    annotationCanonical + "\n# Title",
		byteOrderMark + "# A\r\nB\n": byteOrderMark + annotationCanonical + "\r\n# A\r\nB\n",
	} {
		if got := insertAnnotation(content); got != want {
			t.Fatalf("insertAnnotation(%q) = %q, want %q", content, got, want)
		}
	}
}

// annotateFixture returns a project with an unannotated current
// specification, an annotated one, and an unannotated active change.
func annotateFixture(t *testing.T) string {
	t.Helper()
	root := fixtureRoot(t)
	writeFixture(t, root, "openspec/specs/plain/spec.md", "# plain Specification\n")
	writeFixture(t, root, "openspec/specs/marked/spec.md", annotationCanonical+"\n# marked Specification\n")
	writeFixture(t, root, "openspec/changes/example/specs/plain/spec.md", "## ADDED Requirements\n")
	writeFixture(t, root, "openspec/changes/archive/2026-01-01-old/specs/old/spec.md", "## ADDED Requirements\n")
	return root
}

// countAnnotationWrites counts the files annotate writes.
func countAnnotationWrites(t *testing.T) *int {
	t.Helper()
	original := annotationFileWriter
	t.Cleanup(func() { annotationFileWriter = original })
	writes := 0
	annotationFileWriter = func(name string, data []byte, mode fs.FileMode) error {
		writes++
		return original(name, data, mode)
	}
	return &writes
}

// @verifies scn.specannotation.6bd9c9356646.unit
func TestAnnotateSecondRunChangesNothing(t *testing.T) {
	root := annotateFixture(t)
	writes := countAnnotationWrites(t)
	code, stdout, stderr := runAnnotate(t, "--root", root, "--all")
	if code != 0 || *writes != 2 || !strings.Contains(stdout, "annotated 2 specification files") {
		t.Fatalf("first run = %d, %d writes, %q, %q", code, *writes, stdout, stderr)
	}
	if strings.Contains(readTestFile(t, root, "openspec/changes/archive/2026-01-01-old/specs/old/spec.md"), "stele:") {
		t.Fatal("annotate edited an archived change")
	}
	*writes = 0
	code, stdout, stderr = runAnnotate(t, "--root", root, "--all", "--json")
	var result AnnotationResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatal(err)
	}
	if code != 0 || *writes != 0 || result.Verdict != "pass" || len(result.Files) != 3 {
		t.Fatalf("second run = %d, %d writes, %q, %q", code, *writes, stdout, stderr)
	}
	for _, file := range result.Files {
		if file.State != annotationAnnotated || file.Changed || file.Version == nil || *file.Version != "v1" {
			t.Fatalf("second run file = %#v", file)
		}
	}
	code, stdout, _ = runAnnotate(t, "--root", root, "--all")
	if code != 0 || !strings.Contains(stdout, "every specification file has a Stele annotation (3 files)") {
		t.Fatalf("human second run = %d, %q", code, stdout)
	}
}

// @verifies scn.specannotation.db2f08beff7a.unit
func TestAnnotateLeavesBrokenAnnotations(t *testing.T) {
	root := annotateFixture(t)
	broken := map[string]string{
		"openspec/specs/malformed/spec.md":                 "<!-- stele: spec -->\n# malformed\n",
		"openspec/specs/unsupported/spec.md":               "<!-- stele: spec v9 -->\n# unsupported\n",
		"openspec/changes/example/specs/misplaced/spec.md": "# misplaced\n\n" + annotationCanonical + "\n",
	}
	for path, content := range broken {
		writeFixture(t, root, path, content)
	}
	code, stdout, stderr := runAnnotate(t, "--root", root, "--all")
	if code != 1 || !strings.Contains(stdout, "annotated 2 specification files") {
		t.Fatalf("annotate = %d, %q, %q", code, stdout, stderr)
	}
	for path, content := range broken {
		if readTestFile(t, root, path) != content {
			t.Fatalf("annotate changed %s", path)
		}
	}
	for _, want := range []string{
		"MALFORMED openspec/specs/malformed/spec.md",
		"UNSUPPORTED openspec/specs/unsupported/spec.md",
		"MISPLACED openspec/changes/example/specs/misplaced/spec.md",
		"annotated openspec/specs/plain/spec.md",
		"annotated openspec/changes/example/specs/plain/spec.md",
		"✗ some specification files lack a valid Stele annotation",
	} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("output lacks %q: %q", want, stdout)
		}
	}
	for _, path := range []string{"openspec/specs/plain/spec.md", "openspec/changes/example/specs/plain/spec.md"} {
		if !strings.HasPrefix(readTestFile(t, root, path), annotationCanonical+"\n") {
			t.Fatalf("%s was not annotated", path)
		}
	}
}

func TestAnnotateReportsInvocationAndFilesystemFailures(t *testing.T) {
	root := fixtureRoot(t)
	if code, _, stderr := runAnnotate(t, "--root", root); code != 2 || !strings.Contains(stderr, "no OpenSpec change") {
		t.Fatalf("annotate without a change = %d, %q", code, stderr)
	}
	if code, _, stderr := runAnnotate(t, "--root", root, "--all"); code != 2 ||
		!strings.Contains(stderr, "no active changes to annotate") {
		t.Fatalf("annotate --all without scopes = %d, %q", code, stderr)
	}
	if code, _, stderr := runAnnotate(t, "--root", root, "--specs"); code != 2 ||
		!strings.Contains(stderr, "no current specifications") {
		t.Fatalf("annotate --specs without specs = %d, %q", code, stderr)
	}

	root = annotateFixture(t)
	failure := errors.New("disk full")
	original := annotationFileWriter
	t.Cleanup(func() { annotationFileWriter = original })
	annotationFileWriter = func(string, []byte, fs.FileMode) error { return failure }
	if code, _, stderr := runAnnotate(t, "--root", root, "--specs"); code != 2 ||
		!strings.Contains(stderr, "disk full") {
		t.Fatalf("annotate with a failing write = %d, %q", code, stderr)
	}
	annotationFileWriter = original

	originalRead := readSpecFile
	t.Cleanup(func() { readSpecFile = originalRead })
	readSpecFile = func(string) ([]byte, error) { return nil, failure }
	if code, _, _ := runAnnotate(t, "--root", root, "--specs"); code != 2 {
		t.Fatalf("annotate with a failing read = %d", code)
	}
	readSpecFile = originalRead
	if strings.Contains(readTestFile(t, root, "openspec/specs/plain/spec.md"), "stele:") {
		t.Fatal("a failing run wrote a file")
	}
}

// @verifies scn.specannotation.b7e05fe76741.integration
func TestArchiveRepairRestoresTheAnnotation(t *testing.T) {
	root := openSpecOnlyProject(t)
	steleInit(t, root)
	writeFixture(t, root, "openspec/specs/existing/spec.md", annotationCanonical+`
# existing Specification

## Purpose
Keep the existing behavior of the demo capability stable.

## Requirements
### Requirement: Old
Verification-ID: req.existing.111111111111
The system SHALL keep old behavior.

#### Scenario: Old works
Verification-ID: scn.existing.222222222222
- **WHEN** it is used
- **THEN** it works
`)
	writeFixture(t, root, "openspec/changes/grow/proposal.md", `## Why
Grow the demo with a fresh capability and one more existing requirement.

## What Changes
- Add the fresh capability and extend the existing one.
`)
	writeFixture(t, root, "openspec/changes/grow/tasks.md", "- [x] 1.1 Implement it\n")
	for capability, ids := range map[string][2]string{
		"fresh":    {"req.fresh.333333333333", "scn.fresh.444444444444"},
		"existing": {"req.existing.555555555555", "scn.existing.666666666666"},
	} {
		writeFixture(t, root, "openspec/changes/grow/specs/"+capability+"/spec.md", annotationCanonical+`
## ADDED Requirements

### Requirement: New `+capability+`
Verification-ID: `+ids[0]+`
The system SHALL do new `+capability+` things.

#### Scenario: New `+capability+` works
Verification-ID: `+ids[1]+`
- **WHEN** it is used
- **THEN** it works
`)
	}
	openSpec(t, root, "archive", "grow", "--yes")
	if strings.HasPrefix(readTestFile(t, root, "openspec/specs/fresh/spec.md"), annotationCanonical) {
		t.Fatal("OpenSpec now keeps the annotation on a new specification; revisit the repair step")
	}

	if code, stdout, stderr := runAnnotate(t, "--root", root, "--specs"); code != 0 ||
		!strings.Contains(stdout, "annotated 1 specification files") {
		t.Fatalf("annotate --specs = %d, %q, %q", code, stdout, stderr)
	}
	for _, capability := range []string{"fresh", "existing"} {
		content := readTestFile(t, root, "openspec/specs/"+capability+"/spec.md")
		if !strings.HasPrefix(content, annotationCanonical+"\n") || strings.Count(content, annotationCanonical) != 1 {
			t.Fatalf("%s does not start with exactly one annotation:\n%s", capability, content)
		}
	}
	specs, err := parseScopeSpecs(root, verificationScope{currentSpecs: true})
	if err != nil || len(annotationDiagnostics(specs)) != 0 || len(specs.Requirements) != 3 {
		t.Fatalf("the current specifications = %#v, %v", specs.Diagnostics, err)
	}
}
