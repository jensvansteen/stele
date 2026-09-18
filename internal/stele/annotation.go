package stele

import (
	"errors"
	"fmt"
	"io"
	"regexp"
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
}

// annotationClassifier classifies a file's annotation line by line, so the
// spec parser and the annotate command share one reading of the grammar.
type annotationClassifier struct {
	annotation SpecAnnotation
	// fields lists the ignored fields of the first line, and misplaced the
	// later lines that consist only of an annotation.
	fields    []string
	misplaced []int
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
		classifier.fields = append(classifier.fields, annotationFieldName(field))
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
			fmt.Sprintf("Field %q in the Stele annotation of %s is ignored: format %s defines no fields.",
				field, path, annotationVersion), path, 1, ""))
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
	body := strings.TrimPrefix(content, byteOrderMark)
	bom := content[:len(content)-len(body)]
	terminator := "\n"
	if lines := splitLinesKeepEnds(body); len(lines) > 0 && lineTerminator(lines[0]) != "" {
		terminator = lineTerminator(lines[0])
	}
	return bom + annotationCanonical + terminator + body
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

// planAnnotation reads one file and plans the insertion of a missing annotation.
func planAnnotation(root, scope, file string, check bool) (plannedAnnotation, error) {
	content, err := readSpecFile(file)
	if err != nil {
		return plannedAnnotation{}, err
	}
	path := repositoryPath(root, file)
	annotation := classifyAnnotation(path, string(content)).result()
	planned := plannedAnnotation{file: file, entry: annotationEntry(scope, annotation)}
	if annotation.State == annotationMissing && !check {
		planned.content = insertAnnotation(string(content))
		planned.entry.Changed = true
	}
	return planned, nil
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
func annotateScopes(root string, scopes []verificationScope, check bool) (AnnotationResult, error) {
	result := AnnotationResult{
		SchemaVersion: 1,
		Mode:          choose(check, "check", "write"),
		Verdict:       "pass",
		Files:         []AnnotationFile{},
	}
	planned := make([]plannedAnnotation, 0)
	for _, scope := range scopes {
		name, _, _ := scopeName(scope)
		for _, file := range scope.spec().SpecFiles(root, scope) {
			plan, err := planAnnotation(root, name, file, check)
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
	scopes := everyScope(parsed.root, parsed.backend)
	if len(scopes) == 0 {
		return nil, errors.New("no current specifications and no active changes to annotate")
	}
	return scopes, nil
}

func annotateCommand(parsed options, stdout, stderr io.Writer) int {
	scopes, err := annotateTargetScopes(parsed)
	if err != nil {
		return writeCommandError(stderr, err)
	}
	result, err := annotateScopes(parsed.root, scopes, parsed.check)
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
