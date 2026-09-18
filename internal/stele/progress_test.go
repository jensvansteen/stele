package stele

import (
	"bytes"
	"io"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeClock advances one second every time it is read.
func fakeClock(t *testing.T) {
	t.Helper()
	original := now
	t.Cleanup(func() { now = original })
	current := time.Date(2026, 9, 18, 12, 0, 0, 0, time.UTC)
	now = func() time.Time {
		current = current.Add(time.Second)
		return current
	}
}

// fakeTerminal makes every writer a terminal with the given environment.
func fakeTerminal(t *testing.T, terminal bool, environment map[string]string) {
	t.Helper()
	originalTerminal, originalEnv := isTerminal, lookupEnv
	t.Cleanup(func() { isTerminal, lookupEnv = originalTerminal, originalEnv })
	isTerminal = func(io.Writer) bool { return terminal }
	lookupEnv = func(name string) (string, bool) {
		value, found := environment[name]
		return value, found
	}
}

func progressEvents() []progressEvent {
	return []progressEvent{
		{
			group: "default", path: "tests/a.test.mts", selector: "adds",
			evidenceIDs: []string{"scn.a.aaaaaaaaaaaa.unit"},
			title:       "Add a todo", level: "unit", outcome: "passed",
		},
		{
			group: "default", path: "tests/a.test.mts", selector: "rejects",
			evidenceIDs: []string{"scn.a.bbbbbbbbbbbb.unit"}, title: "Reject empty text", level: "unit",
			outcome: "failed",
		},
		{group: "default", path: "tests/b.test.mts", selector: "exports", title: "Export", outcome: "passed"},
	}
}

func runProgress(progress progressReporter) {
	events := progressEvents()
	progress.stage("OpenSpec strict validation")
	progress.stageDone(markPass, "OpenSpec strict validation: passed")
	progress.planned(len(events), map[string]int{"tests/a.test.mts": 2, "tests/b.test.mts": 1})
	for _, event := range events {
		progress.started(event)
		progress.finished(event)
	}
	progress.done()
}

// @verifies scn.terminalreport.a735c55107ce.unit
func TestTerminalProgressUpdatesOneLine(t *testing.T) {
	fakeClock(t)
	fakeTerminal(t, true, map[string]string{"COLUMNS": "200"})
	var stderr bytes.Buffer
	progress := newProgress(&stderr, false, "", reportStyle{})
	if _, ok := progress.(*terminalProgress); !ok {
		t.Fatalf("a terminal got %T", progress)
	}
	runProgress(progress)
	output := stderr.String()
	assertOrdered(t, output,
		clearLine+"⠋ OpenSpec strict validation…",
		clearLine+"✓ OpenSpec strict validation: passed (1.0s)\n",
		clearLine+"⠙ tests 0/3",
		"tests 0/3 · Add a todo (unit) · 0 ✓ 0 ✗ · 0:02",
		"tests 1/3 · Add a todo (unit) · 1 ✓ 0 ✗",
		clearLine+"✗ tests/a.test.mts rejects (scn.a.bbbbbbbbbbbb.unit)\n",
		"tests 2/3 · Reject empty text (unit) · 1 ✓ 1 ✗",
		"tests 3/3 · Export · 2 ✓ 1 ✗",
	)
	if !strings.HasSuffix(output, clearLine) {
		t.Fatalf("the progress line was not cleared: %q", output)
	}
	for line := range strings.SplitSeq(output, "\r") {
		if len([]rune(strings.TrimPrefix(line, "\x1b[2K"))) > 200+1 {
			t.Fatalf("a progress line exceeds the width: %q", line)
		}
	}

	narrow := &terminalProgress{state: newProgressState(&stderr, "[change x] ", reportStyle{}), width: 20}
	stderr.Reset()
	narrow.planned(2, map[string]int{"a": 2})
	narrow.started(progressEvent{path: "a", selector: "one", title: "A long scenario title"})
	narrow.started(progressEvent{path: "a", selector: "two", title: "Another"})
	if line := strings.TrimPrefix(stderr.String()[strings.LastIndex(stderr.String(), "\r"):], clearLine); len(
		[]rune(line)) != 20 || !strings.HasSuffix(line, "…") {
		t.Fatalf("a narrow terminal line = %q", line)
	}
	medium := &terminalProgress{state: newProgressState(&stderr, "", reportStyle{}), width: 44}
	medium.planned(2, map[string]int{"a": 2})
	medium.started(progressEvent{path: "a", selector: "one", title: "A long scenario title", level: "unit"})
	if line := stderr.String()[strings.LastIndex(stderr.String(), "\r"):]; !strings.HasSuffix(line,
		"⠙ tests 0/2 · A long scena… · 0 ✓ 0 ✗ · 0:02") {
		t.Fatalf("the title was not shortened first: %q", line)
	}
	wide := &terminalProgress{state: newProgressState(&stderr, "", reportStyle{}), width: 200}
	wide.planned(2, map[string]int{"a": 2})
	wide.started(progressEvent{path: "a", selector: "one"})
	wide.started(progressEvent{path: "a", selector: "two"})
	if !strings.Contains(stderr.String(), "2 running") {
		t.Fatalf("concurrent tests are not counted: %q", stderr.String())
	}
}

// @verifies scn.terminalreport.3fc2e84a5b98.unit
func TestLineProgressPrintsPlainLines(t *testing.T) {
	fakeClock(t)
	fakeTerminal(t, false, map[string]string{})
	var stderr bytes.Buffer
	progress := newProgress(&stderr, false, "[change x] ", reportStyle{color: true})
	runProgress(progress)
	want := "[change x] OpenSpec strict validation: passed (1.0s)\n" +
		"[change x] test execution: 3 tests in 2 files\n" +
		"[change x]   FAILED tests/a.test.mts rejects (scn.a.bbbbbbbbbbbb.unit)\n" +
		"[change x]   ✗ tests/a.test.mts  1/2 (1.0s) [2/3]\n" +
		"[change x]   ✓ tests/b.test.mts  1/1 (1.0s) [3/3]\n" +
		"[change x] test execution: 2/3 passed (5.0s)\n"
	if stderr.String() != want {
		t.Fatalf("plain progress =\n%s\nwant\n%s", stderr.String(), want)
	}
	if strings.Contains(stderr.String(), "\x1b") {
		t.Fatal("plain progress contains escape sequences")
	}
	fakeTerminal(t, true, map[string]string{"TERM": "dumb"})
	if _, ok := newProgress(&stderr, false, "", reportStyle{}).(*lineProgress); !ok {
		t.Fatal("a dumb terminal got in-place progress")
	}
	if _, ok := newProgress(&stderr, true, "", reportStyle{}).(silentProgress); !ok {
		t.Fatal("quiet progress is not silent")
	}
	runProgress(silentProgress{})
	if terminalWidth() != 80 {
		t.Fatal("the default width is not 80 columns")
	}
}

func TestProgressIsSafeForConcurrentUse(t *testing.T) {
	fakeTerminal(t, false, map[string]string{})
	var stderr lockedBuffer
	progress := newProgress(&stderr, false, "", reportStyle{})
	events := make(map[string]int)
	for index := range 50 {
		events["tests/"+string(rune('a'+index%5))+".test.mts"]++
	}
	progress.planned(50, events)
	var group sync.WaitGroup
	for index := range 50 {
		group.Add(1)
		go func() {
			defer group.Done()
			event := progressEvent{
				path:     "tests/" + string(rune('a'+index%5)) + ".test.mts",
				selector: strings.Repeat("x", index+1), slot: index % 4, outcome: "passed",
			}
			progress.started(event)
			progress.finished(event)
		}()
	}
	group.Wait()
	for line := range strings.SplitSeq(strings.TrimSuffix(stderr.String(), "\n"), "\n") {
		if !strings.HasPrefix(line, "test execution:") && !strings.HasPrefix(line, "  ✓ tests/") {
			t.Fatalf("an interleaved line: %q", line)
		}
	}
	if !strings.Contains(stderr.String(), "test execution: 50/50 passed") {
		t.Fatalf("the final line is missing:\n%s", stderr.String())
	}
}

// lockedBuffer is a buffer safe for concurrent writes.
type lockedBuffer struct {
	mutex  sync.Mutex
	buffer bytes.Buffer
}

func (buffer *lockedBuffer) Write(content []byte) (int, error) {
	buffer.mutex.Lock()
	defer buffer.mutex.Unlock()
	return buffer.buffer.Write(content)
}

func (buffer *lockedBuffer) String() string {
	return buffer.buffer.String()
}
