package stele

import (
	"fmt"
	"io"
	"strings"
)

// colorModes are the accepted values of --color.
var colorModes = map[string]bool{"auto": true, "always": true, "never": true}

// colorEnabled decides color for one stream: always and never are explicit,
// and auto colors a terminal unless NO_COLOR is set or TERM is dumb.
func colorEnabled(mode string, writer io.Writer) bool {
	switch mode {
	case "always":
		return true
	case "never":
		return false
	}
	noColor, _ := lookupEnv("NO_COLOR")
	term, _ := lookupEnv("TERM")
	return noColor == "" && term != "dumb" && isTerminal(writer)
}

// style returns the report style for a stream.
func (parsed options) style(writer io.Writer) reportStyle {
	return reportStyle{color: colorEnabled(parsed.color, writer), details: parsed.details}
}

// progress returns the progress output for standard error, naming the scope
// during an --all run.
func (parsed options) progress(stderr io.Writer) progressReporter {
	prefix := ""
	if parsed.every != nil {
		_, _, label := scopeName(resolveScope(parsed))
		prefix = "[" + label + "] "
	}
	return newProgress(stderr, parsed.quiet, prefix, parsed.style(stderr))
}

// writeReport prints a command's human report, or with --quiet only its
// verdict line, and records the verdict for an --all summary. --details and
// --color reach the renderer through the style.
//
// @implements req.terminalreport.ebb070079783
func (parsed options) writeReport(stdout io.Writer, input humanReportInput) {
	report := buildHumanReport(input)
	parsed.every.recordVerdict(report)
	style := parsed.style(stdout)
	if parsed.quiet {
		if parsed.every == nil {
			renderVerdictOnly(stdout, report, style)
		}
		return
	}
	renderHumanReport(stdout, report, style)
}

// recordVerdict keeps a scope's verdict for the summary of an --all run.
func (run *everyScopeRun) recordVerdict(report humanReport) {
	if run != nil {
		*run.verdicts = append(*run.verdicts, report)
	}
}

// renderScopeSummary prints the summary of an --all run: every scope's verdict
// and one final line.
//
// @implements req.terminalreport.2cda6fd7e60c
func renderScopeSummary(stdout io.Writer, verdicts []humanReport, failed []string, scopes int, parsed options) {
	style := parsed.style(stdout)
	final := humanReport{passed: len(failed) == 0, verdict: fmt.Sprintf("PASSED  all %d scopes passed", scopes)}
	if !final.passed {
		final.verdict = fmt.Sprintf("FAILED  %d of %d scopes failed: %s", len(failed), scopes,
			strings.Join(failed, ", "))
	}
	if !parsed.quiet {
		_, _ = fmt.Fprintln(stdout, "Scopes")
		for _, verdict := range verdicts {
			_, _ = fmt.Fprintln(stdout, "  "+verdictLineText(verdict, style))
		}
	}
	renderVerdictOnly(stdout, final, style)
}
