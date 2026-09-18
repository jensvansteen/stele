package stele

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

// Seams for progress output: the clock, terminal detection, and the
// environment. Tests replace them.
var (
	now        = time.Now
	isTerminal = func(writer io.Writer) bool {
		file, ok := writer.(*os.File)
		if !ok {
			return false
		}
		info, err := file.Stat()
		return err == nil && info.Mode()&os.ModeCharDevice != 0
	}
	lookupEnv = os.LookupEnv
)

// progressEvent identifies one test for progress output. The group and the
// worker slot let a concurrent runner report several tests at once; the
// sequential runner uses group "default" and slot 0.
type progressEvent struct {
	group       string
	slot        int
	path        string
	selector    string
	evidenceIDs []string
	scenarioIDs []string
	// title and level describe the first scenario and evidence of the test.
	title   string
	level   string
	outcome string
	reason  string
}

// key identifies the test of an event, whatever order events arrive in.
func (event progressEvent) key() string {
	return event.path + "\x00" + event.selector
}

// testObserver receives test progress. It never affects evidence, and its
// methods are safe to call from several goroutines.
type testObserver interface {
	planned(total int, files map[string]int)
	started(event progressEvent)
	finished(event progressEvent)
}

// progressReporter shows stages and tests while a command runs.
type progressReporter interface {
	testObserver
	stage(name string)
	stageDone(mark, text string)
	done()
}

// silentProgress shows nothing, for --quiet and for internal runs.
type silentProgress struct{}

func (silentProgress) planned(int, map[string]int) {}
func (silentProgress) started(progressEvent)       {}
func (silentProgress) finished(progressEvent)      {}
func (silentProgress) stage(string)                {}
func (silentProgress) stageDone(string, string)    {}
func (silentProgress) done()                       {}

// progressState is what both progress styles count. Every method holds the
// mutex, and each output is written with one call, so lines never interleave.
type progressState struct {
	mutex      sync.Mutex
	writer     io.Writer
	prefix     string
	style      reportStyle
	stageName  string
	stageStart time.Time
	testStart  time.Time
	total      int
	finished   int
	passed     int
	failed     int
	files      map[string]int
	fileDone   map[string]int
	fileFailed map[string]int
	fileStart  map[string]time.Time
	running    map[string]progressEvent
	latest     progressEvent
}

func newProgressState(writer io.Writer, prefix string, style reportStyle) progressState {
	return progressState{
		writer: writer, prefix: prefix, style: style, running: map[string]progressEvent{},
		fileDone: map[string]int{}, fileFailed: map[string]int{}, fileStart: map[string]time.Time{},
	}
}

func (state *progressState) write(text string) {
	_, _ = io.WriteString(state.writer, text)
}

// failureText names a failed test and its evidence.
func failureText(event progressEvent) string {
	name := strings.TrimSpace(event.path + " " + event.selector)
	if len(event.evidenceIDs) > 0 {
		name += " (" + strings.Join(event.evidenceIDs, ", ") + ")"
	}
	return name
}

// newProgress chooses the progress style for standard error.
//
// @implements req.terminalreport.a36aac068cc3
func newProgress(stderr io.Writer, quiet bool, prefix string, style reportStyle) progressReporter {
	if quiet {
		return silentProgress{}
	}
	term, _ := lookupEnv("TERM")
	if isTerminal(stderr) && term != "dumb" {
		return &terminalProgress{state: newProgressState(stderr, prefix, style), width: terminalWidth()}
	}
	style.color = false
	return &lineProgress{state: newProgressState(stderr, prefix, style)}
}

// terminalWidth reads COLUMNS, or assumes 80 columns.
func terminalWidth() int {
	value, _ := lookupEnv("COLUMNS")
	if width, err := strconv.Atoi(value); err == nil && width > 0 {
		return width
	}
	return 80
}

// elapsed prints a duration as minutes and seconds.
func elapsed(duration time.Duration) string {
	seconds := int(duration.Seconds())
	return fmt.Sprintf("%d:%02d", seconds/60, seconds%60)
}

// terminalProgress updates one line in place and prints failures above it.
type terminalProgress struct {
	state   progressState
	width   int
	redraws int
}

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

const clearLine = "\r\x1b[2K"

func (progress *terminalProgress) stage(name string) {
	progress.state.mutex.Lock()
	defer progress.state.mutex.Unlock()
	progress.state.stageName, progress.state.stageStart = name, now()
	progress.redraw()
}

func (progress *terminalProgress) stageDone(mark, text string) {
	progress.state.mutex.Lock()
	defer progress.state.mutex.Unlock()
	progress.state.write(fmt.Sprintf("%s%s %s%s (%s)\n", clearLine, progress.state.style.paint(mark),
		progress.state.prefix, text, formatDuration(now().Sub(progress.state.stageStart))))
	progress.state.stageName = ""
}

func (progress *terminalProgress) planned(total int, files map[string]int) {
	progress.state.mutex.Lock()
	defer progress.state.mutex.Unlock()
	progress.state.total, progress.state.files, progress.state.testStart = total, files, now()
	progress.state.stageName = "Test execution"
	progress.redraw()
}

func (progress *terminalProgress) started(event progressEvent) {
	progress.state.mutex.Lock()
	defer progress.state.mutex.Unlock()
	progress.state.running[event.key()] = event
	progress.state.latest = event
	progress.redraw()
}

func (progress *terminalProgress) finished(event progressEvent) {
	progress.state.mutex.Lock()
	defer progress.state.mutex.Unlock()
	delete(progress.state.running, event.key())
	progress.state.finished++
	if event.outcome == "passed" {
		progress.state.passed++
	} else {
		progress.state.failed++
		progress.state.write(clearLine + progress.state.style.paint(markFail) + " " + failureText(event) + "\n")
	}
	progress.redraw()
}

func (progress *terminalProgress) done() {
	progress.state.mutex.Lock()
	defer progress.state.mutex.Unlock()
	progress.state.write(clearLine)
}

// redraw rewrites the progress line from the counters. When the line is too
// wide, the scenario title is shortened first, so the counts and the elapsed
// time stay visible, and then the line is cut to the width.
func (progress *terminalProgress) redraw() {
	state := &progress.state
	frame := spinnerFrames[progress.redraws%len(spinnerFrames)]
	progress.redraws++
	head, middle, tail := frame+" "+state.prefix+state.stageName+"…", "", ""
	if state.total > 0 {
		head = fmt.Sprintf("%s %stests %d/%d", frame, state.prefix, state.finished, state.total)
		if state.latest.path != "" {
			middle = " · " + state.latest.title
			if state.latest.level != "" {
				middle += " (" + state.latest.level + ")"
			}
		}
		if len(state.running) > 1 {
			tail = fmt.Sprintf(" · %d running", len(state.running))
		}
		tail += fmt.Sprintf(" · %d %s %d %s · %s", state.passed, markPass, state.failed, markFail,
			elapsed(now().Sub(state.testStart)))
	}
	room := progress.width - utf8.RuneCountInString(head+tail)
	if utf8.RuneCountInString(middle) > room {
		middle = truncate(middle, max(room, 1))
	}
	line := head + middle + tail
	if utf8.RuneCountInString(line) > progress.width {
		line = truncate(line, progress.width)
	}
	state.write(clearLine + line)
}

// lineProgress writes append-only lines without escape sequences.
type lineProgress struct {
	state progressState
}

func (progress *lineProgress) stage(name string) {
	progress.state.mutex.Lock()
	defer progress.state.mutex.Unlock()
	progress.state.stageName, progress.state.stageStart = name, now()
}

func (progress *lineProgress) stageDone(_, text string) {
	progress.state.mutex.Lock()
	defer progress.state.mutex.Unlock()
	progress.state.write(fmt.Sprintf("%s%s (%s)\n", progress.state.prefix, text,
		formatDuration(now().Sub(progress.state.stageStart))))
}

func (progress *lineProgress) planned(total int, files map[string]int) {
	progress.state.mutex.Lock()
	defer progress.state.mutex.Unlock()
	progress.state.total, progress.state.files, progress.state.testStart = total, files, now()
	progress.state.write(fmt.Sprintf("%stest execution: %d tests in %d files\n", progress.state.prefix, total,
		len(files)))
}

func (progress *lineProgress) started(event progressEvent) {
	progress.state.mutex.Lock()
	defer progress.state.mutex.Unlock()
	if _, begun := progress.state.fileStart[event.path]; !begun {
		progress.state.fileStart[event.path] = now()
	}
}

func (progress *lineProgress) finished(event progressEvent) {
	progress.state.mutex.Lock()
	defer progress.state.mutex.Unlock()
	state := &progress.state
	state.finished++
	state.fileDone[event.path]++
	lines := ""
	if event.outcome == "passed" {
		state.passed++
	} else {
		state.failed++
		state.fileFailed[event.path]++
		lines += fmt.Sprintf("%s  FAILED %s\n", state.prefix, failureText(event))
	}
	if state.fileDone[event.path] == state.files[event.path] {
		done := state.fileDone[event.path]
		lines += fmt.Sprintf("%s  %s %s  %d/%d (%s) [%d/%d]\n", state.prefix,
			choose(state.fileFailed[event.path] == 0, markPass, markFail), event.path,
			done-state.fileFailed[event.path], done, formatDuration(now().Sub(state.fileStart[event.path])),
			state.finished, state.total)
	}
	if state.finished == state.total {
		lines += fmt.Sprintf("%stest execution: %d/%d passed (%s)\n", state.prefix, state.passed, state.total,
			formatDuration(now().Sub(state.testStart)))
	}
	state.write(lines)
}

func (progress *lineProgress) done() {}
