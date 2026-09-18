package stele

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"
)

var (
	standardInput   io.Reader = os.Stdin
	inputIsTerminal           = stdinIsTerminal
	gitUserName               = configuredGitUserName
	approvalDate              = func() string { return time.Now().Format(time.DateOnly) }
	writePlanFile             = writeJSON
)

var errApprovalNeedsConfirmation = errors.New(
	"stele approve needs a human decision: run it in an interactive terminal to review each entry, " +
		"pass --all --yes to approve every pending entry, or, as an agent, pass --confirmed-in-chat " +
		"only after the human explicitly confirmed the levels in the conversation",
)

// approvalMode says how pending entries are decided.
type approvalMode uint8

const (
	approveInteractively approvalMode = iota
	approveAll
	approveConfirmedInChat
)

type approvalRequest struct {
	root      string
	scope     verificationScope
	evidence  []string
	scenarios []string
	mode      approvalMode
	approver  string
	date      string
	revision  string
	input     io.Reader
	output    io.Writer
}

type approvalResult struct {
	Approved []string
	Rejected []string
}

// planDocument is one v2 plan file together with the scenarios it plans.
type planDocument struct {
	path     string
	file     evidencePlanFile
	modified bool
}

// pendingEntry is an unapproved or stale evidence entry awaiting a decision.
type pendingEntry struct {
	document *planDocument
	scenario Scenario
	index    int
	state    string
}

func stdinIsTerminal() bool {
	info, err := os.Stdin.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

func configuredGitUserName(root string) string {
	command := exec.Command("git", "config", "user.name")
	command.Dir = root
	output, _ := command.Output()
	return strings.TrimSpace(string(output))
}

// approveEvidence records approvals for the pending and stale entries of the
// selected plans. Interactive review asks per entry; the batch and
// conversation modes approve every selected entry.
//
// @implements req.verificationstrategy.c09342159cf0
func approveEvidence(request approvalRequest) (approvalResult, error) {
	result := approvalResult{Approved: []string{}, Rejected: []string{}}
	documents, scenarios, err := loadApprovalDocuments(request.root, request.scope)
	if err != nil {
		return result, err
	}
	reader := bufio.NewScanner(request.input)
	for _, entry := range pendingEntries(documents, scenarios, request) {
		evidence := &entry.document.file.Scenarios[entry.scenario.ID].Evidence[entry.index]
		decision := "approve"
		if request.mode == approveInteractively {
			decision = askDecision(reader, request.output, entry, *evidence)
		}
		switch decision {
		case "approve":
			evidence.Approval = &EvidenceApproval{
				Approver: request.approver,
				Date:     request.date,
				Digest:   evidenceDigest(entry.scenario, *evidence),
				Via:      choose(request.mode == approveConfirmedInChat, "agent-confirmed", "cli"),
				Revision: request.revision,
			}
			entry.document.modified = true
			result.Approved = append(result.Approved, evidence.ID)
		case "reject":
			result.Rejected = append(result.Rejected, evidence.ID)
		}
	}
	for _, document := range documents {
		if !document.modified {
			continue
		}
		if err := writePlanFile(document.path, document.file); err != nil {
			return result, err
		}
	}
	return result, nil
}

// loadApprovalDocuments reads the v2 plans of a scope and the scenarios they
// may approve.
func loadApprovalDocuments(root string, scope verificationScope) ([]*planDocument, map[string]Scenario, error) {
	if err := requireScopeSpecs(root, scope); err != nil {
		return nil, nil, err
	}
	parsed, err := parseScopeSpecs(root, scope)
	if err != nil {
		return nil, nil, err
	}
	scenarios := make(map[string]Scenario)
	for _, requirement := range parsed.Requirements {
		for _, scenario := range requirement.Scenarios {
			scenarios[scenario.ID] = scenario
		}
	}
	paths := []string{filepath.Join(root, "openspec", "changes", scope.changeID, linkagePlanFile)}
	if scope.currentSpecs {
		paths = walkFiles(filepath.Join(root, "openspec", "changes", "archive"), func(path string) bool {
			return filepath.Base(path) == linkagePlanFile
		})
	}
	documents := make([]*planDocument, 0, len(paths))
	for _, path := range paths {
		if !fileExists(path) {
			continue
		}
		document, err := readPlanDocument(path, scope.currentSpecs)
		if err != nil {
			return nil, nil, err
		}
		if document != nil {
			documents = append(documents, document)
		}
	}
	if len(documents) == 0 {
		return nil, nil, errors.New(
			"no v2 linkage plan found; write one with the stele-plan skill or run stele plan migrate",
		)
	}
	return documents, scenarios, nil
}

// readPlanDocument reads a v2 plan for approval. Archived v1 plans are skipped;
// a change plan must be v2.
func readPlanDocument(path string, skipV1 bool) (*planDocument, error) {
	plan, err := readLinkagePlan(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", filepath.ToSlash(path), err)
	}
	if plan.SchemaVersion != evidencePlanVersion {
		if skipV1 {
			return nil, nil
		}
		return nil, fmt.Errorf("%s uses schema version 1; run stele plan migrate first", filepath.ToSlash(path))
	}
	document := &planDocument{path: path, file: evidencePlanFile{
		SchemaVersion: evidencePlanVersion,
		ChangeID:      plan.ChangeID,
		Scenarios:     map[string]ScenarioEvidence{},
	}}
	for id, entries := range plan.Evidence {
		document.file.Scenarios[id] = ScenarioEvidence{Evidence: entries}
	}
	return document, nil
}

// pendingEntries lists the selected unapproved and stale entries in a stable
// order. Entries of undeclared scenarios cannot be approved.
func pendingEntries(documents []*planDocument, scenarios map[string]Scenario, request approvalRequest) []pendingEntry {
	pending := make([]pendingEntry, 0)
	for _, document := range documents {
		ids := make([]string, 0, len(document.file.Scenarios))
		for id := range document.file.Scenarios {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		for _, id := range ids {
			scenario, declared := scenarios[id]
			if !declared || (len(request.scenarios) > 0 && !slices.Contains(request.scenarios, id)) {
				continue
			}
			for index, entry := range document.file.Scenarios[id].Evidence {
				state := approvalState(scenario, entry)
				if state == "approved" || evidenceEntryProblem(scenario, entry, map[string]bool{}) != "" ||
					(len(request.evidence) > 0 && !slices.Contains(request.evidence, entry.ID)) {
					continue
				}
				pending = append(pending, pendingEntry{
					document: document,
					scenario: scenario,
					index:    index,
					state:    state,
				})
			}
		}
	}
	return pending
}

// askDecision shows one entry and reads approve, reject, or skip. The end of
// input skips the remaining entries.
func askDecision(reader *bufio.Scanner, output io.Writer, entry pendingEntry, evidence EvidenceEntry) string {
	_, _ = fmt.Fprintf(output, "\n%s %s (%s)\n", entry.scenario.ID, entry.scenario.Title, entry.state)
	_, _ = fmt.Fprintf(output, "  %s\n", strings.ReplaceAll(strings.TrimSpace(entry.scenario.Text), "\n", "\n  "))
	_, _ = fmt.Fprintf(output, "Evidence:  %s\nLevel:     %s\nRationale: %s\n", evidence.ID, evidence.Level,
		evidence.Rationale)
	if evidence.Placement != "" {
		_, _ = fmt.Fprintf(output, "Placement: %s (advisory)\n", evidence.Placement)
	}
	for {
		_, _ = fmt.Fprint(output, "Approve, reject, or skip? [a/r/s] ")
		if !reader.Scan() {
			_, _ = fmt.Fprintln(output)
			return "skip"
		}
		switch strings.ToLower(strings.TrimSpace(reader.Text())) {
		case "a", "approve", "y", "yes":
			return "approve"
		case "r", "reject":
			return "reject"
		case "", "s", "skip":
			return "skip"
		}
	}
}

func approveCommand(parsed options, stdout, stderr io.Writer) int {
	mode, err := selectApprovalMode(parsed)
	if err != nil {
		return writeCommandError(stderr, err)
	}
	approver := parsed.approver
	if approver == "" {
		approver = gitUserName(parsed.root)
	}
	if approver == "" {
		return writeCommandError(stderr, errors.New("no approver: set git config user.name or pass --by NAME"))
	}
	revision, _ := gitState(parsed.root)
	result, err := approveEvidence(approvalRequest{
		root:      parsed.root,
		scope:     resolveScope(parsed),
		evidence:  parsed.evidenceIDs,
		scenarios: parsed.scenarioIDs,
		mode:      mode,
		approver:  approver,
		date:      approvalDate(),
		revision:  choose(revision == "uncommitted", "", revision),
		input:     standardInput,
		output:    stdout,
	})
	if err != nil {
		return writeCommandError(stderr, err)
	}
	renderApprovals(stdout, result, mode, approver)
	return 0
}

func selectApprovalMode(parsed options) (approvalMode, error) {
	switch {
	case parsed.yes && !parsed.all:
		return 0, errors.New("--yes requires --all")
	case parsed.confirmedInChat && parsed.all:
		return 0, errors.New("--confirmed-in-chat cannot be combined with --all")
	case parsed.confirmedInChat:
		return approveConfirmedInChat, nil
	case parsed.all && parsed.yes:
		return approveAll, nil
	case inputIsTerminal():
		return approveInteractively, nil
	default:
		return 0, errApprovalNeedsConfirmation
	}
}

func renderApprovals(stdout io.Writer, result approvalResult, mode approvalMode, approver string) {
	if len(result.Approved) == 0 && len(result.Rejected) == 0 {
		_, _ = fmt.Fprintln(stdout, "No pending or stale evidence entries were decided.")
		return
	}
	via := choose(mode == approveConfirmedInChat, "agent-confirmed", "cli")
	for _, id := range result.Approved {
		_, _ = fmt.Fprintf(stdout, "✓ approved %s by %s (via %s)\n", id, approver, via)
	}
	if len(result.Rejected) > 0 {
		_, _ = fmt.Fprintf(stdout, "Rejected, revise these entries and ask again: %s\n",
			strings.Join(result.Rejected, ", "))
	}
}
