package stele

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

// The annotation that marks a Stele specification file. Stele always writes
// the canonical form; readers accept the grammar in docs/concepts/spec-format.md.
const (
	annotationCanonical = "<!-- stele: spec v1 -->"
	annotationVersion   = "v1"
	byteOrderMark       = "\xef\xbb\xbf"
)

// Annotation states of a specification file.
const (
	annotationAnnotated   = "annotated"
	annotationMissing     = "missing"
	annotationMisplaced   = "misplaced"
	annotationMalformed   = "malformed"
	annotationUnsupported = "unsupported"
)

// Values of the unannotatedSpecs setting. The default is warn until 0.2.0,
// when it becomes error; warn then stays accepted as an explicit choice.
const (
	unannotatedWarn    = "warn"
	unannotatedError   = "error"
	unannotatedDefault = unannotatedWarn
)

var (
	// annotationClaimPattern matches a first line that claims to be a Stele
	// annotation. It is also the trigger pattern editors use.
	annotationClaimPattern = regexp.MustCompile(`^[ \t]*<!--[ \t]*stele:`)
	annotationPattern      = regexp.MustCompile(
		`^[ \t]*<!--[ \t]*stele:[ \t]*spec[ \t]+(v[0-9]+)((?:[ \t]*;[^;]*?)*)[ \t]*-->[ \t]*$`)
	annotationFieldPattern = regexp.MustCompile(`^[ \t]*([a-z][a-z0-9-]*)[ \t]*:[ \t]*(.*?)[ \t]*$`)
	annotationFileWriter   = writeSpecFile
)

// SpecAnnotation is the annotation state of one specification file. Version is
// the declared version of an annotated or unsupported file and otherwise empty.
type SpecAnnotation struct {
	Path    string
	State   string
	Version string
	// Targets lists the valid names of the targets field in the written
	// order, and TargetsDeclared tells whether the field is present.
	Targets         []string
	TargetsDeclared bool
	// fields keeps every field of the first line as written, trimmed.
	fields []string
}

// annotationClassifier classifies a file's annotation line by line, so the
// spec parser and the annotate command share one reading of the grammar.
type annotationClassifier struct {
	annotation SpecAnnotation
	// fields lists the ignored fields of the first line, and misplaced the
	// later lines that consist only of an annotation.
	fields    []string
	misplaced []int
	// targetProblems explains a malformed or repeated targets field.
	targetProblems []string
}

func newAnnotationClassifier(path string) *annotationClassifier {
	return &annotationClassifier{annotation: SpecAnnotation{Path: path, State: annotationMissing}}
}

// line classifies one line of the file, numbered from 1, without its terminator.
//
// @implements req.specannotation.331a671f3614
func (classifier *annotationClassifier) line(number int, text string) {
	if number > 1 {
		if classifier.annotation.State == annotationMissing && annotationPattern.MatchString(text) {
			classifier.misplaced = append(classifier.misplaced, number)
		}
		return
	}
	text = strings.TrimPrefix(text, byteOrderMark)
	if !annotationClaimPattern.MatchString(text) {
		return
	}
	match := annotationPattern.FindStringSubmatch(text)
	if match == nil {
		classifier.annotation.State = annotationMalformed
		return
	}
	classifier.annotation.Version = match[1]
	classifier.annotation.State = choose(match[1] == annotationVersion, annotationAnnotated, annotationUnsupported)
	// Text before the first ";" is whitespace; every part after one is a field.
	for _, field := range strings.Split(match[2], ";")[1:] {
		classifier.annotation.fields = append(classifier.annotation.fields, strings.TrimSpace(field))
		if value, isTargets := targetsField(field); isTargets {
			classifier.readTargets(value)
			continue
		}
		classifier.fields = append(classifier.fields, annotationFieldName(field))
	}
}

// targetsField reports whether a field is the targets field, and its value.
func targetsField(field string) (string, bool) {
	match := annotationFieldPattern.FindStringSubmatch(field)
	if match == nil || strings.Contains(match[2], "--") || match[1] != "targets" {
		return "", false
	}
	return match[2], true
}

// readTargets reads the targets field; version 1 allows it once.
//
// @implements req.specannotation.331a671f3614
func (classifier *annotationClassifier) readTargets(value string) {
	if classifier.annotation.TargetsDeclared {
		classifier.targetProblems = append(classifier.targetProblems, "the targets field appears more than once")
		return
	}
	names, problem := parseTargetList(value)
	classifier.annotation.TargetsDeclared = true
	classifier.annotation.Targets = names
	if problem != "" {
		classifier.targetProblems = append(classifier.targetProblems, "the targets list is malformed: "+problem)
	}
}

// annotationFieldName names a field in a warning: its key when it follows
// `key: value`, and otherwise its trimmed text.
func annotationFieldName(field string) string {
	if match := annotationFieldPattern.FindStringSubmatch(field); match != nil && !strings.Contains(match[2], "--") {
		return match[1]
	}
	return strings.TrimSpace(field)
}

// result returns the file's state: a misplaced annotation replaces missing.
func (classifier *annotationClassifier) result() SpecAnnotation {
	annotation := classifier.annotation
	if annotation.State == annotationMissing && len(classifier.misplaced) > 0 {
		annotation.State = annotationMisplaced
	}
	return annotation
}

// diagnostics reports the file's annotation problems. Missing and misplaced
// annotations are warnings here; the scope's policy sets their severity.
func (classifier *annotationClassifier) diagnostics(fix string) []Diagnostic {
	annotation := classifier.result()
	path := annotation.Path
	diagnostics := make([]Diagnostic, 0)
	switch annotation.State {
	case annotationMissing:
		diagnostics = append(diagnostics, diagnostic("SPEC_ANNOTATION_MISSING", "warning",
			fmt.Sprintf("%s has no Stele annotation on its first line; run `%s` to add `%s`.",
				path, fix, annotationCanonical), path, 1, ""))
	case annotationMisplaced:
		for _, line := range classifier.misplaced {
			diagnostics = append(diagnostics, diagnostic("SPEC_ANNOTATION_MISPLACED", "warning",
				fmt.Sprintf("The Stele annotation on line %d of %s must be the first line; "+
					"move it there, and the file counts as unannotated until then.", line, path), path, line, ""))
		}
	case annotationMalformed:
		diagnostics = append(diagnostics, diagnostic("SPEC_ANNOTATION_MALFORMED", "error",
			fmt.Sprintf("The first line of %s starts a Stele annotation but does not match `%s`.",
				path, annotationCanonical), path, 1, ""))
	case annotationUnsupported:
		diagnostics = append(diagnostics, diagnostic("SPEC_ANNOTATION_UNSUPPORTED", "error",
			fmt.Sprintf("%s declares Stele specification format %s, which Stele %s does not support; "+
				"upgrade Stele to verify it.", path, annotation.Version, Version), path, 1, ""))
	}
	for _, field := range classifier.fields {
		diagnostics = append(diagnostics, diagnostic("SPEC_ANNOTATION_FIELD_IGNORED", "warning",
			fmt.Sprintf("Field %q in the Stele annotation of %s is ignored: format %s defines only the targets field.",
				field, path, annotationVersion), path, 1, ""))
	}
	if annotation.State == annotationAnnotated {
		for _, problem := range classifier.targetProblems {
			diagnostics = append(diagnostics, diagnostic("SPEC_TARGETS_MALFORMED", "error",
				fmt.Sprintf("In the Stele annotation of %s, %s.", path, problem), path, 1, ""))
		}
	}
	return diagnostics
}

// classifyAnnotation classifies the annotation of a file's content.
func classifyAnnotation(path, content string) *annotationClassifier {
	classifier := newAnnotationClassifier(path)
	for index, line := range splitLinesKeepEnds(content) {
		classifier.line(index+1, lineText(line))
	}
	return classifier
}

// insertAnnotation adds the canonical annotation as the first line, after a
// byte order mark, ending it like the file's first line or with a line feed.
// Every other byte is kept.
func insertAnnotation(content string) string {
	return insertTargetedAnnotation(content, nil)
}

// insertTargetedAnnotation inserts the annotation like insertAnnotation, with
// a targets field when targets are given.
func insertTargetedAnnotation(content string, targets []string) string {
	body := strings.TrimPrefix(content, byteOrderMark)
	bom := content[:len(content)-len(body)]
	terminator := "\n"
	if lines := splitLinesKeepEnds(body); len(lines) > 0 && lineTerminator(lines[0]) != "" {
		terminator = lineTerminator(lines[0])
	}
	return bom + annotationLine(targets, nil) + terminator + body
}

// annotationLine writes the canonical annotation with an optional targets
// field followed by other fields as written.
func annotationLine(targets []string, others []string) string {
	if targets == nil && len(others) == 0 {
		return annotationCanonical
	}
	var line strings.Builder
	line.WriteString("<!-- stele: spec " + annotationVersion)
	if targets != nil {
		line.WriteString("; targets: " + strings.Join(targets, ", "))
	}
	for _, field := range others {
		line.WriteString("; " + field)
	}
	line.WriteString(" -->")
	return line.String()
}

// annotationFix names the command that adds a scope's missing annotations.
func annotationFix(scope verificationScope) string {
	if scope.currentSpecs {
		return "stele annotate --specs"
	}
	return fmt.Sprintf("stele annotate --change %s` or `stele ids --change %s", scope.changeID, scope.changeID)
}

// annotationSeverity returns the severity a policy gives a missing annotation.
func annotationSeverity(policy string) string {
	if policy == "" {
		policy = unannotatedDefault
	}
	return choose(policy == unannotatedError, "error", "warning")
}

// applyAnnotationPolicy sets the severity of missing and misplaced annotations.
//
// @implements req.specannotation.ec94d82dea47
func applyAnnotationPolicy(diagnostics []Diagnostic, policy string) {
	for index := range diagnostics {
		switch diagnostics[index].Code {
		case "SPEC_ANNOTATION_MISSING", "SPEC_ANNOTATION_MISPLACED":
			diagnostics[index].Severity = annotationSeverity(policy)
		}
	}
}

// validateAnnotationPolicy rejects a policy other than warn and error.
func validateAnnotationPolicy(policy string) error {
	switch policy {
	case "", unannotatedWarn, unannotatedError:
		return nil
	}
	return fmt.Errorf("invalid unannotatedSpecs %q in stele.config.json; accepted values: %s, %s",
		policy, unannotatedWarn, unannotatedError)
}

// AnnotationFile is one specification file of an annotate or ids run. State is
// the state Stele found, and Changed tells whether it added the annotation.
type AnnotationFile struct {
	Scope   string  `json:"scope,omitempty"`
	Path    string  `json:"path"`
	State   string  `json:"state"`
	Version *string `json:"version"`
	Changed bool    `json:"changed"`
}

// AnnotationResult reports the annotations of the files in the selected scopes.
type AnnotationResult struct {
	SchemaVersion int              `json:"schemaVersion"`
	Mode          string           `json:"mode"`
	Verdict       string           `json:"verdict"`
	Files         []AnnotationFile `json:"files"`
}

// plannedAnnotation is a file's annotation and, when it changes, its new content.
type plannedAnnotation struct {
	file    string
	entry   AnnotationFile
	content string
}

// planAnnotation reads one file and plans the insertion of a missing
// annotation, with the targets of the capability's current specification
// when the file is a delta spec. With targetsFrom, it also plans setting the
// targets of a current specification from the archived delta spec.
func planAnnotation(root string, scope verificationScope, file, targetsFrom string, check bool) (plannedAnnotation,
	error,
) {
	content, err := readSpecFile(file)
	if err != nil {
		return plannedAnnotation{}, err
	}
	name, _, _ := scopeName(scope)
	path := repositoryPath(root, file)
	annotation := classifyAnnotation(path, string(content)).result()
	planned := plannedAnnotation{file: file, entry: annotationEntry(name, annotation)}
	if check {
		return planned, nil
	}
	targets := capabilityTargets(root, scope, scope.spec().Capability(scope, path))
	if targetsFrom != "" {
		var archived bool
		if targets, archived = archivedTargets(targetsFrom, capabilityOf(path)); !archived {
			targets = annotation.Targets
			if !annotation.TargetsDeclared {
				targets = nil
			}
		}
	}
	switch {
	case annotation.State == annotationMissing:
		planned.content = insertTargetedAnnotation(string(content), targets)
		planned.entry.Changed = true
	case annotation.State == annotationAnnotated && targetsFrom != "" &&
		(annotation.TargetsDeclared != (targets != nil) || !slices.Equal(annotation.Targets, targets)):
		planned.content = replaceAnnotationTargets(string(content), annotation, targets)
		planned.entry.Changed = true
	}
	return planned, nil
}

// capabilityTargets returns the targets that a capability's current
// specification declares, for a delta spec of a change, or nil.
func capabilityTargets(root string, scope verificationScope, capability string) []string {
	if scope.currentSpecs || capability == "" {
		return nil
	}
	content, err := readSpecFile(scope.spec().CurrentSpecFile(root, capability))
	if err != nil {
		return nil
	}
	annotation := classifyAnnotation("", string(content)).result()
	if !annotation.TargetsDeclared {
		return nil
	}
	return annotation.Targets
}

// archivedTargets reads the targets of a capability's delta spec in an
// archived change directory: nil when it declares none, and false when the
// archive has no delta spec for the capability.
func archivedTargets(directory, capability string) ([]string, bool) {
	content, err := readSpecFile(filepath.Join(directory, "specs", capability, "spec.md"))
	if err != nil {
		return nil, false
	}
	annotation := classifyAnnotation("", string(content)).result()
	if !annotation.TargetsDeclared {
		return nil, true
	}
	return append([]string{}, annotation.Targets...), true
}

// replaceAnnotationTargets rewrites the first line with the given targets,
// or without a targets field for nil, keeping every other field and every
// other byte.
//
// @implements req.specannotation.1707277552af
func replaceAnnotationTargets(content string, annotation SpecAnnotation, targets []string) string {
	body := strings.TrimPrefix(content, byteOrderMark)
	bom := content[:len(content)-len(body)]
	lines := splitLinesKeepEnds(body)
	others := make([]string, 0, len(annotation.fields))
	for _, field := range annotation.fields {
		if _, isTargets := targetsField(field); !isTargets {
			others = append(others, field)
		}
	}
	return bom + annotationLine(targets, others) + lineTerminator(lines[0]) + strings.Join(lines[1:], "")
}

func annotationEntry(scope string, annotation SpecAnnotation) AnnotationFile {
	var version *string
	if annotation.Version != "" {
		value := annotation.Version
		version = &value
	}
	return AnnotationFile{Scope: scope, Path: annotation.Path, State: annotation.State, Version: version}
}

// writeAnnotations writes every planned insertion, after every file was read.
func writeAnnotations(planned []plannedAnnotation) error {
	for _, plan := range planned {
		if !plan.entry.Changed {
			continue
		}
		if err := annotationFileWriter(plan.file, []byte(plan.content), 0o644); err != nil {
			return err
		}
	}
	return nil
}

// annotateScopes classifies every specification file of the scopes, and unless
// checking, adds the annotation to the files that lack one. Files with a
// misplaced, malformed, or unsupported annotation are never edited.
//
// @implements req.specannotation.c4d7843868f5
func annotateScopes(root string, scopes []verificationScope, check bool, targetsFrom string) (AnnotationResult,
	error,
) {
	result := AnnotationResult{
		SchemaVersion: 1,
		Mode:          choose(check, "check", "write"),
		Verdict:       "pass",
		Files:         []AnnotationFile{},
	}
	planned := make([]plannedAnnotation, 0)
	for _, scope := range scopes {
		for _, file := range scope.spec().SpecFiles(root, scope) {
			plan, err := planAnnotation(root, scope, file, targetsFrom, check)
			if err != nil {
				return result, err
			}
			planned = append(planned, plan)
			result.Files = append(result.Files, plan.entry)
			if plan.entry.State != annotationAnnotated && !plan.entry.Changed {
				result.Verdict = "fail"
			}
		}
	}
	if check {
		return result, nil
	}
	return result, writeAnnotations(planned)
}

// annotateTargetScopes returns the scopes of an annotate run: every scope with
// --all, and otherwise the selected change or the current specifications.
func annotateTargetScopes(parsed options) ([]verificationScope, error) {
	if !parsed.allScopes {
		scope := resolveScope(parsed)
		return []verificationScope{scope}, requireScopeSpecs(parsed.root, scope)
	}
	scopes := everyScope(parsed.root, parsed.baseScope())
	if len(scopes) == 0 {
		return nil, errors.New("no current specifications and no active changes to annotate")
	}
	return scopes, nil
}

func annotateCommand(parsed options, stdout, stderr io.Writer) int {
	targetsFrom, err := targetsFromDirectory(parsed)
	if err != nil {
		return writeCommandError(stderr, err)
	}
	scopes, err := annotateTargetScopes(parsed)
	if err != nil {
		return writeCommandError(stderr, err)
	}
	result, err := annotateScopes(parsed.root, scopes, parsed.check, targetsFrom)
	if err != nil {
		return writeCommandError(stderr, err)
	}
	if parsed.json {
		writeMachineJSON(stdout, result)
	} else {
		renderAnnotations(stdout, result)
	}
	return resultExitCode(result.Verdict == "pass")
}

// targetsFromDirectory resolves --targets-from: an archived change directory,
// only with --specs.
func targetsFromDirectory(parsed options) (string, error) {
	if parsed.targetsFrom == "" {
		return "", nil
	}
	if !parsed.specs {
		return "", errors.New("--targets-from needs --specs: it sets the targets of the current specifications")
	}
	directory := resolveWithin(parsed.root, parsed.targetsFrom)
	if info, err := os.Stat(directory); err != nil || !info.IsDir() {
		return "", fmt.Errorf("--targets-from %s is not a directory", parsed.targetsFrom)
	}
	return directory, nil
}

// annotationProblem explains a state that a person has to fix.
var annotationProblem = map[string]string{
	annotationMissing:     "has no Stele annotation",
	annotationMisplaced:   "has its Stele annotation below the first line; move it to line 1",
	annotationMalformed:   "has a malformed Stele annotation on line 1; fix it by hand",
	annotationUnsupported: "declares an unsupported Stele format version; upgrade Stele",
}

func renderAnnotations(stdout io.Writer, result AnnotationResult) {
	changed := 0
	for _, file := range result.Files {
		if file.Changed {
			changed++
		}
	}
	switch {
	case changed > 0:
		_, _ = fmt.Fprintf(stdout, "✓ annotated %d specification files\n", changed)
	case result.Verdict == "pass":
		_, _ = fmt.Fprintf(stdout, "✓ every specification file has a Stele annotation (%d files)\n",
			len(result.Files))
	}
	renderAnnotationFiles(stdout, result.Files)
	if result.Verdict == "fail" {
		_, _ = fmt.Fprintln(stdout, "✗ some specification files lack a valid Stele annotation")
	}
}

// renderAnnotationFiles lists each annotated file and each file with a problem.
func renderAnnotationFiles(stdout io.Writer, files []AnnotationFile) {
	for _, file := range files {
		switch {
		case file.Changed:
			_, _ = fmt.Fprintf(stdout, "  annotated %s\n", file.Path)
		case file.State != annotationAnnotated:
			_, _ = fmt.Fprintf(stdout, "  %s %s %s\n", strings.ToUpper(file.State), file.Path,
				annotationProblem[file.State])
		}
	}
}
