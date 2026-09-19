package stele

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// checkStep is one step of `stele check` in its JSON document.
type checkStep struct {
	Step     string          `json:"step"`
	ExitCode int             `json:"exitCode"`
	Result   json.RawMessage `json:"result"`
	Error    string          `json:"error,omitempty"`
}

// checkResult is the JSON document of `stele check`.
type checkResult struct {
	SchemaVersion int         `json:"schemaVersion"`
	Verdict       string      `json:"verdict"`
	ExitCode      int         `json:"exitCode"`
	Steps         []checkStep `json:"steps"`
}

// checkStepSummary is the human summary line of one step.
type checkStepSummary struct {
	name, detail string
	code         int
	lines        []string
	// annotations locate the step's findings for GitHub Actions.
	annotations []annotation
}

// checkCommand runs the ID check, the annotation check, and validation for
// the selected scope, every step even when an earlier one fails, and exits
// with the worst code.
//
// @implements req.validate.70d1435b3ff3
func checkCommand(parsed options, stdout, stderr io.Writer) int {
	scopes := []verificationScope{resolveScope(parsed)}
	if parsed.allScopes {
		scopes = everyScope(parsed.root, parsed.baseScope())
	}
	identities, identitiesStep := checkIdentities(parsed.root, scopes)
	annotations, annotationsStep := checkAnnotations(parsed.root, scopes)
	parsed.annotations.addAll(identities.annotations)
	parsed.annotations.addAll(annotations.annotations)
	var validation bytes.Buffer
	validate := parsed
	validate.color = choose(colorEnabled(parsed.color, stdout), "always", "never")
	validationCode := routeCommand("validate", validate, &validation, stderr)
	validationStep := checkStep{Step: "validate", ExitCode: validationCode, Result: rawJSON(validation.Bytes())}
	steps := []checkStep{identitiesStep, annotationsStep, validationStep}
	worst := 0
	for _, step := range steps {
		worst = max(worst, step.ExitCode)
	}
	if parsed.json {
		writeMachineJSON(stdout, checkResult{
			SchemaVersion: 1, Verdict: choose(worst == 0, "pass", "fail"),
			ExitCode: worst, Steps: steps,
		})
		return worst
	}
	summaries := []checkStepSummary{
		identities, annotations,
		{
			name: "Validation", code: validationCode,
			detail: choose(validationCode == 0, "passed", "see the report below"),
		},
	}
	renderCheck(stdout, parsed, scopes, summaries, validation.String(), worst)
	return worst
}

func rawJSON(content []byte) json.RawMessage {
	if json.Valid(content) {
		return json.RawMessage(content)
	}
	return json.RawMessage("null")
}

// checkIdentities checks the Verification-IDs of every scope, including the
// current specifications, without writing.
func checkIdentities(root string, scopes []verificationScope) (checkStepSummary, checkStep) {
	summary := checkStepSummary{name: "Verification-IDs"}
	results := make([]IdentityResult, 0, len(scopes))
	missing := 0
	for _, scope := range scopes {
		result, err := assignScopeIdentities(root, scope, true)
		if err != nil {
			summary.code, summary.detail = 2, err.Error()
			return summary, checkStep{Step: "ids", ExitCode: 2, Result: json.RawMessage("null"), Error: err.Error()}
		}
		results = append(results, result)
		if result.Verdict != "pass" {
			summary.code = 1
		}
		name, _, _ := scopeName(scope)
		for _, insertion := range result.Insertions {
			missing++
			summary.annotations = append(summary.annotations, identityAnnotation(insertion, scopeFlag(scope, name)))
			summary.lines = append(summary.lines, fmt.Sprintf("%s:%d %s %q", insertion.Path, insertion.Line,
				insertion.Kind, insertion.Title))
		}
	}
	summary.detail = fmt.Sprintf("%s; every requirement and scenario has an ID", plural(len(scopes), "scope"))
	if missing > 0 {
		summary.detail = fmt.Sprintf("%s without a Verification-ID; run `stele ids`", plural(missing, "heading"))
	}
	content, _ := MarshalDeterministic(results)
	return summary, checkStep{Step: "ids", ExitCode: summary.code, Result: content}
}

// checkAnnotations checks the Stele annotation of every specification file.
func checkAnnotations(root string, scopes []verificationScope) (checkStepSummary, checkStep) {
	summary := checkStepSummary{name: "Annotations"}
	result, err := annotateScopes(root, scopes, true, "")
	if err != nil {
		summary.code, summary.detail = 2, err.Error()
		return summary, checkStep{Step: "annotate", ExitCode: 2, Result: json.RawMessage("null"), Error: err.Error()}
	}
	summary.detail = fmt.Sprintf("%s annotated", plural(len(result.Files), "file"))
	for _, file := range result.Files {
		if file.State != annotationAnnotated {
			summary.code = 1
			summary.lines = append(summary.lines, fmt.Sprintf("%s %s", strings.ToUpper(file.State), file.Path))
			summary.annotations = append(summary.annotations,
				fileAnnotation(file, choose(file.Scope == "specs", "--specs", "--change "+file.Scope)))
		}
	}
	if summary.code != 0 {
		summary.detail = fmt.Sprintf("%s without a valid annotation; run `stele annotate`",
			plural(len(summary.lines), "file"))
	}
	content, _ := MarshalDeterministic(result)
	return summary, checkStep{Step: "annotate", ExitCode: summary.code, Result: content}
}

func renderCheck(stdout io.Writer, parsed options, scopes []verificationScope, steps []checkStepSummary,
	validation string, worst int,
) {
	style := parsed.style(stdout)
	label := "all scopes"
	if !parsed.allScopes {
		_, _, label = scopeName(scopes[0])
	}
	failed := make([]string, 0)
	for _, step := range steps {
		if step.code != 0 {
			failed = append(failed, strings.ToLower(step.name))
		}
	}
	verdict := humanReport{passed: worst == 0, verdict: "PASSED  check · " + label}
	if worst != 0 {
		verdict.verdict = "FAILED  check · " + label + " — " + strings.Join(failed, ", ")
	}
	if parsed.quiet {
		renderVerdictOnly(stdout, verdict, style)
		return
	}
	var out strings.Builder
	fmt.Fprintf(&out, "stele %s · check · %s\n\n", Version, label)
	for _, step := range steps {
		fmt.Fprintf(&out, "  %s %s%s\n", style.paint(choose(step.code == 0, markPass, markFail)), pad(step.name, 20),
			step.detail)
		for _, line := range step.lines {
			out.WriteString("      " + line + "\n")
		}
	}
	out.WriteString("\n" + validation + "\n")
	_, _ = io.WriteString(stdout, out.String())
	renderVerdictOnly(stdout, verdict, style)
}
