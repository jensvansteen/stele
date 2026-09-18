package stele

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"
)

// annotationModes are the accepted values of --annotations.
var annotationModes = map[string]bool{"auto": true, "github": true, "never": true}

// annotation is one GitHub Actions workflow command: an error or a warning,
// with a root-relative file and line when the location is known.
type annotation struct {
	level, path string
	line        int
	title       string
	message     string
}

// annotationSink collects the annotations of one command, so they are written
// once, after the run, in report order and without repeated lines. A nil sink
// collects nothing.
type annotationSink struct {
	prefix string
	lines  []string
	seen   map[string]bool
}

// newAnnotationSink returns a sink when annotations are on: always with
// github, never with never, and with auto only in GitHub Actions.
//
// @implements req.terminalreport.b475f4f66880
func newAnnotationSink(mode, root string) *annotationSink {
	enabled := mode == "github"
	if mode == "auto" {
		value, _ := lookupEnv("GITHUB_ACTIONS")
		enabled = value == "true"
	}
	if !enabled {
		return nil
	}
	return &annotationSink{prefix: workspacePrefix(root), seen: map[string]bool{}}
}

// workspacePrefix returns the location of the project root within
// GITHUB_WORKSPACE, with a trailing slash, or "" when the root is the
// workspace or lies outside it.
func workspacePrefix(root string) string {
	workspace, _ := lookupEnv("GITHUB_WORKSPACE")
	if workspace == "" {
		return ""
	}
	relative, err := filepath.Rel(resolvedPath(workspace), resolvedPath(root))
	relative = filepath.ToSlash(relative)
	if err != nil || relative == "." || relative == ".." || strings.HasPrefix(relative, "../") {
		return ""
	}
	return relative + "/"
}

// resolvedPath follows symbolic links, so a workspace and a root reached
// through different links still compare.
func resolvedPath(path string) string {
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		return resolved
	}
	return path
}

// add records one annotation unless the same line was already recorded.
func (sink *annotationSink) add(item annotation) {
	if sink == nil {
		return
	}
	path := item.path
	if path != "" {
		path = sink.prefix + path
	}
	line := workflowCommand(item.level, path, item.line, item.title, item.message)
	if !sink.seen[line] {
		sink.seen[line] = true
		sink.lines = append(sink.lines, line)
	}
}

// addAll records annotations in order.
func (sink *annotationSink) addAll(items []annotation) {
	for _, item := range items {
		sink.add(item)
	}
}

// addReport records the findings and failed tests that the report shows,
// and one location-free annotation for each truncated group.
func (sink *annotationSink) addReport(report humanReport, details bool) {
	style := reportStyle{details: details}
	for _, group := range report.groups {
		level := choose(group.severity == "warning", "warning", "error")
		shown := limited(len(group.items), style)
		for _, item := range group.items[:shown] {
			sink.add(annotation{
				level: level, path: item.path, line: item.line, title: group.code,
				message: joinNonEmpty(" ", group.meaning, joinNonEmpty(" · ", item.id, item.title)),
			})
		}
		if hidden := len(group.items) - shown; hidden > 0 {
			sink.add(annotation{level: level, title: group.code, message: moreText(hidden, "more finding")})
		}
	}
	shown := limited(len(report.failed), style)
	for _, test := range report.failed[:shown] {
		sink.add(annotation{
			level: "error", path: test.path, line: test.line, title: "Test failed",
			message: joinNonEmpty(" · ", test.name, test.detail),
		})
	}
	if hidden := len(report.failed) - shown; hidden > 0 {
		sink.add(annotation{level: "error", title: "Test failed", message: moreText(hidden, "more failed test")})
	}
}

func moreText(hidden int, noun string) string {
	return plural(hidden, noun) + " not shown; run with --details to list every one"
}

// flush writes the recorded annotations to standard error.
func (sink *annotationSink) flush(stderr io.Writer) {
	if sink == nil || len(sink.lines) == 0 {
		return
	}
	_, _ = io.WriteString(stderr, strings.Join(sink.lines, "\n")+"\n")
}

// workflowCommand renders one annotation as a GitHub Actions workflow
// command: ::error file=F,line=L,title=T::message.
func workflowCommand(level, path string, line int, title, message string) string {
	properties := make([]string, 0, 3)
	if path != "" {
		properties = append(properties, "file="+escapeProperty(path))
		if line > 0 {
			properties = append(properties, fmt.Sprintf("line=%d", line))
		}
	}
	properties = append(properties, "title="+escapeProperty(title))
	return "::" + level + " " + strings.Join(properties, ",") + "::" + escapeData(message)
}

// escapeData escapes a workflow command message as GitHub's toolkit does.
func escapeData(value string) string {
	return strings.NewReplacer("%", "%25", "\r", "%0D", "\n", "%0A").Replace(value)
}

// escapeProperty escapes a workflow command property value as GitHub's
// toolkit does.
func escapeProperty(value string) string {
	return strings.NewReplacer(":", "%3A", ",", "%2C").Replace(escapeData(value))
}

func joinNonEmpty(separator string, values ...string) string {
	kept := make([]string, 0, len(values))
	for _, value := range values {
		if value != "" {
			kept = append(kept, value)
		}
	}
	return strings.Join(kept, separator)
}

// annotate records a command's report for annotations.
func (parsed options) annotate(input humanReportInput) {
	if parsed.annotations != nil {
		parsed.annotations.addReport(buildHumanReport(input), parsed.details)
	}
}

// identityAnnotation annotates a heading that lacks a Verification-ID.
func identityAnnotation(insertion IdentityInsertion, flag string) annotation {
	code := choose(insertion.Kind == "requirement", "ID_REQUIREMENT_MISSING", "ID_SCENARIO_MISSING")
	guide := guideFor(code)
	return annotation{
		level: "error", path: insertion.Path, line: insertion.Line, title: code,
		message: fmt.Sprintf("%s %s %q; %s", guide.meaning, insertion.Kind, insertion.Title, guide.fixFor(flag)),
	}
}

// fileAnnotation annotates a specification file without a valid annotation.
func fileAnnotation(file AnnotationFile, flag string) annotation {
	code := "SPEC_ANNOTATION_" + strings.ToUpper(file.State)
	guide := guideFor(code)
	return annotation{
		level: "error", path: file.Path, title: code,
		message: guide.meaning + " Fix: " + guide.fixFor(flag),
	}
}
