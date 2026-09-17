package stele

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

const identityDerivationVersion = "stele-ids/v1"

var (
	deltaSectionPattern   = regexp.MustCompile(`^##[ \t]+(?:(?i)(ADDED|MODIFIED|REMOVED|RENAMED)[ \t]+Requirements)?`)
	renameFromPattern     = regexp.MustCompile("^\\s*[-*+]?\\s*FROM:\\s*`?###\\s*Requirement:\\s*(.+?)`?\\s*$")
	renameToPattern       = regexp.MustCompile("^\\s*[-*+]?\\s*TO:\\s*`?###\\s*Requirement:\\s*(.+?)`?\\s*$")
	identityTokenPattern  = regexp.MustCompile(`\b(?:req|scn)\.[a-z0-9]+\.[a-f0-9]{12}\b`)
	namespaceStripPattern = regexp.MustCompile(`[^a-z0-9]+`)
	readSpecFile          = os.ReadFile
	writeSpecFile         = os.WriteFile
	scanIdentityAnchors   = ScanAnchors
)

// identitySlot is one requirement or scenario heading as the verifier sees it.
type identitySlot struct {
	line        int
	kind        identityTarget
	requirement string
	scenario    string
	section     string
	hasID       bool
	occurrence  int
}

// specDocument is the identity-relevant structure of one delta spec file.
type specDocument struct {
	lines     []string
	slots     []*identitySlot
	renames   map[string]string
	namespace string
}

// identityRegistry holds every token a newly derived identity must avoid.
type identityRegistry struct {
	taken  map[string]bool
	change map[string]bool
}

type plannedSpecFile struct {
	path    string
	content []byte
	changed bool
}

// AssignIdentities inserts a Verification-ID below every requirement and scenario
// heading of a change's delta specs that has none. With check set it only reports
// the headings that lack one and writes nothing.
//
// @implements req.ids.5cd09e44308f
func AssignIdentities(root, changeID string, check bool) (IdentityResult, error) {
	result := IdentityResult{
		SchemaVersion: 1,
		ChangeID:      changeID,
		Mode:          choose(check, "check", "write"),
		Verdict:       "pass",
		Insertions:    []IdentityInsertion{},
	}
	scope := changeScope(changeID)
	if err := requireScopeSpecs(root, scope); err != nil {
		return result, err
	}
	files := walkFiles(scope.specsRoot(root), func(path string) bool {
		return strings.EqualFold(filepath.Ext(path), ".md")
	})
	registry, err := newIdentityRegistry(root, files)
	if err != nil {
		return result, err
	}

	planned := make([]plannedSpecFile, 0, len(files))
	for _, file := range files {
		relative, err := relativePath(root, file)
		if err != nil {
			return result, err
		}
		plan, insertions, err := planSpecFile(root, changeID, filepath.ToSlash(relative), file, registry, check)
		if err != nil {
			return result, err
		}
		planned = append(planned, plan)
		result.Insertions = append(result.Insertions, insertions...)
	}

	if check {
		result.Verdict = choose(len(result.Insertions) == 0, "pass", "fail")
		return result, nil
	}
	for _, plan := range planned {
		if !plan.changed {
			continue
		}
		if err := writeSpecFile(plan.path, plan.content, 0o644); err != nil {
			return result, err
		}
	}
	return result, nil
}

// newIdentityRegistry collects every identity token under openspec/ and every
// anchored identity, so a new ID never silently links to existing behavior or to
// a leftover anchor.
func newIdentityRegistry(root string, changeFiles []string) (identityRegistry, error) {
	registry := identityRegistry{taken: map[string]bool{}, change: map[string]bool{}}
	openSpecFiles := walkFiles(filepath.Join(root, "openspec"), func(path string) bool {
		return strings.EqualFold(filepath.Ext(path), ".md")
	})
	if err := collectIdentityTokens(openSpecFiles, registry.taken); err != nil {
		return registry, err
	}
	if err := collectIdentityTokens(changeFiles, registry.change); err != nil {
		return registry, err
	}
	anchors, err := scanIdentityAnchors(root)
	if err != nil {
		return registry, err
	}
	for _, anchor := range anchors {
		registry.taken[anchor.ID] = true
	}
	return registry, nil
}

func collectIdentityTokens(files []string, into map[string]bool) error {
	for _, file := range files {
		content, err := readSpecFile(file)
		if err != nil {
			return err
		}
		for _, token := range identityTokenPattern.FindAllString(string(content), -1) {
			into[token] = true
		}
	}
	return nil
}

func (registry identityRegistry) reserve(identity string) {
	registry.taken[identity] = true
	registry.change[identity] = true
}

func planSpecFile(
	root string,
	changeID string,
	relative string,
	file string,
	registry identityRegistry,
	check bool,
) (plannedSpecFile, []IdentityInsertion, error) {
	content, err := readSpecFile(file)
	if err != nil {
		return plannedSpecFile{}, nil, err
	}
	capability := specCapability(changeID, relative)
	document := parseSpecDocument(string(content))
	namespace := document.namespace
	if namespace == "" {
		namespace = capabilityNamespace(capability)
	}
	base, err := loadBaseIdentities(root, capability, document)
	if err != nil {
		return plannedSpecFile{}, nil, err
	}

	insertions := make([]IdentityInsertion, 0)
	insertAfter := make(map[int]string)
	for _, slot := range document.slots {
		if slot.hasID || slot.section == "REMOVED" || slot.section == "RENAMED" {
			continue
		}
		identity, origin := base.reuse(slot, document.renames, registry)
		if identity == "" {
			identity = deriveIdentity(slot, changeID, capability, namespace, registry.taken)
		}
		registry.reserve(identity)
		insertAfter[slot.line] = identity
		line := slot.line + 1
		if !check {
			line += len(insertions) + 1
		}
		insertions = append(insertions, IdentityInsertion{
			ID:     identity,
			Kind:   choose(slot.kind == requirementIdentityTarget, "requirement", "scenario"),
			Title:  choose(slot.kind == requirementIdentityTarget, slot.requirement, slot.scenario),
			Path:   relative,
			Line:   line,
			Origin: origin,
		})
	}
	plan := plannedSpecFile{path: file, content: content, changed: len(insertAfter) > 0}
	if plan.changed {
		plan.content = []byte(insertIdentityLines(document.lines, insertAfter))
	}
	return plan, insertions, nil
}

// splitLinesKeepEnds splits content into lines that keep their own terminators,
// so joining them reproduces the original bytes exactly.
func splitLinesKeepEnds(content string) []string {
	lines := make([]string, 0)
	for content != "" {
		index := strings.IndexByte(content, '\n')
		if index < 0 {
			lines = append(lines, content)
			break
		}
		lines = append(lines, content[:index+1])
		content = content[index+1:]
	}
	return lines
}

func lineTerminator(line string) string {
	switch {
	case strings.HasSuffix(line, "\r\n"):
		return "\r\n"
	case strings.HasSuffix(line, "\n"):
		return "\n"
	default:
		return ""
	}
}

func lineText(line string) string {
	return strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r")
}

// specDocumentParser mirrors the verifier's heading and identity rules, so the
// command inserts exactly the identities that verification would report missing.
type specDocumentParser struct {
	document      specDocument
	occurrences   map[string]int
	section       string
	pendingRename string
	requirement   string
	current       *identitySlot
}

func parseSpecDocument(content string) specDocument {
	parser := specDocumentParser{
		document:    specDocument{lines: splitLinesKeepEnds(content), renames: map[string]string{}},
		occurrences: map[string]int{},
	}
	for index, raw := range parser.document.lines {
		parser.parseLine(index, lineText(raw))
	}
	return parser.document
}

func (parser *specDocumentParser) parseLine(index int, text string) {
	if match := requirementPattern.FindStringSubmatch(text); match != nil {
		parser.requirement = strings.TrimSpace(match[1])
		parser.addSlot(index, requirementIdentityTarget, "")
		return
	}
	if parser.current == nil {
		// Like the verifier, ignore scenarios and IDs that precede every requirement.
		parser.parseDeltaLine(text)
		return
	}
	if match := scenarioPattern.FindStringSubmatch(text); match != nil {
		parser.addSlot(index, scenarioIdentityTarget, strings.TrimSpace(match[1]))
		return
	}
	if match := verificationIDPattern.FindStringSubmatch(text); match != nil {
		parser.current.hasID = true
		if parser.document.namespace == "" && identityPattern.MatchString(match[1]) {
			parser.document.namespace = strings.Split(match[1], ".")[1]
		}
		return
	}
	parser.parseDeltaLine(text)
}

func (parser *specDocumentParser) parseDeltaLine(text string) {
	if match := deltaSectionPattern.FindStringSubmatch(text); match != nil {
		parser.section = strings.ToUpper(match[1])
		return
	}
	if parser.section != "RENAMED" {
		return
	}
	if match := renameFromPattern.FindStringSubmatch(text); match != nil {
		parser.pendingRename = strings.TrimSpace(match[1])
		return
	}
	if match := renameToPattern.FindStringSubmatch(text); match != nil && parser.pendingRename != "" {
		parser.document.renames[strings.TrimSpace(match[1])] = parser.pendingRename
		parser.pendingRename = ""
	}
}

func (parser *specDocumentParser) addSlot(index int, kind identityTarget, scenario string) {
	slot := &identitySlot{
		line:        index,
		kind:        kind,
		requirement: parser.requirement,
		scenario:    scenario,
		section:     parser.section,
	}
	key := fmt.Sprintf("%d\x00%s\x00%s", kind, slot.requirement, scenario)
	slot.occurrence = parser.occurrences[key]
	parser.occurrences[key]++
	parser.document.slots = append(parser.document.slots, slot)
	parser.current = slot
}

func insertIdentityLines(lines []string, insertAfter map[int]string) string {
	fallback := "\n"
	for _, line := range lines {
		if terminator := lineTerminator(line); terminator != "" {
			fallback = terminator
			break
		}
	}
	var builder strings.Builder
	for index, line := range lines {
		identity, found := insertAfter[index]
		if !found {
			builder.WriteString(line)
			continue
		}
		terminator := lineTerminator(line)
		if terminator == "" {
			// The heading ends the file: keep the file without a final newline.
			builder.WriteString(line + fallback + "Verification-ID: " + identity)
			continue
		}
		builder.WriteString(line + "Verification-ID: " + identity + terminator)
	}
	return builder.String()
}

// specCapability returns the capability path of a delta spec, such as "todo" for
// specs/todo/spec.md or "platform/auth" for specs/platform/auth/spec.md.
func specCapability(changeID, relative string) string {
	withinSpecs := strings.TrimPrefix(relative, "openspec/changes/"+changeID+"/specs/")
	directory := path.Dir(withinSpecs)
	if directory == "." {
		return strings.TrimSuffix(withinSpecs, path.Ext(withinSpecs))
	}
	return directory
}

// capabilityNamespace reduces a capability path to the identity namespace grammar
// [a-z0-9]+, so "platform/auth-tokens" becomes "platformauthtokens".
func capabilityNamespace(capability string) string {
	namespace := namespaceStripPattern.ReplaceAllString(strings.ToLower(capability), "")
	if namespace == "" {
		return "spec"
	}
	return namespace
}

// deriveIdentity derives the 12-hex token from the change, capability path,
// heading kind, titles, and the heading's occurrence in its file. A derived token,
// unlike a random one, makes the command reproducible and testable. Written tokens
// never change, so later title edits do not move them. The attempt counter skips
// tokens that already exist.
//
// @implements req.ids.320cb32c3b8a
func deriveIdentity(slot *identitySlot, changeID, capability, namespace string, taken map[string]bool) string {
	prefix := choose(slot.kind == requirementIdentityTarget, "req", "scn")
	for attempt := 0; ; attempt++ {
		hash := sha256.New()
		for _, part := range []string{
			identityDerivationVersion,
			changeID,
			capability,
			prefix,
			slot.requirement,
			slot.scenario,
			strconv.Itoa(slot.occurrence),
			strconv.Itoa(attempt),
		} {
			_, _ = hash.Write([]byte(part))
			_, _ = hash.Write([]byte{0})
		}
		identity := fmt.Sprintf("%s.%s.%s", prefix, namespace, hex.EncodeToString(hash.Sum(nil))[:12])
		if !taken[identity] {
			return identity
		}
	}
}

// baseIdentities maps the current specification's requirement titles and
// requirement+scenario titles to their identities.
type baseIdentities map[string]string

func baseKey(requirement, scenario string) string {
	return requirement + "\x00" + scenario
}

// loadBaseIdentities reads the identities that a MODIFIED section must keep.
//
// @implements req.ids.877eabdee0b4
func loadBaseIdentities(root, capability string, document specDocument) (baseIdentities, error) {
	base := baseIdentities{}
	needed := false
	for _, slot := range document.slots {
		needed = needed || (!slot.hasID && slot.section == "MODIFIED")
	}
	if !needed {
		return base, nil
	}
	file := filepath.Join(root, "openspec", "specs", filepath.FromSlash(capability), "spec.md")
	content, err := readSpecFile(file)
	if errors.Is(err, os.ErrNotExist) {
		return base, nil
	}
	if err != nil {
		return nil, err
	}
	parser := specFileParser{path: file}
	for _, line := range splitLinesKeepEnds(string(content)) {
		parser.line++
		parser.parseLine(lineText(line))
	}
	for _, requirement := range parser.requirements {
		base.record(baseKey(requirement.Title, ""), requirement.ID, "req.")
		for _, scenario := range requirement.Scenarios {
			base.record(baseKey(requirement.Title, scenario.Title), scenario.ID, "scn.")
		}
	}
	return base, nil
}

func (base baseIdentities) record(key, identity, prefix string) {
	if _, exists := base[key]; exists {
		return
	}
	if identityPattern.MatchString(identity) && strings.HasPrefix(identity, prefix) {
		base[key] = identity
	}
}

// reuse keeps the identity of behavior that a MODIFIED section changes.
func (base baseIdentities) reuse(
	slot *identitySlot,
	renames map[string]string,
	registry identityRegistry,
) (string, string) {
	if slot.section != "MODIFIED" {
		return "", "generated"
	}
	requirement := slot.requirement
	if previous, renamed := renames[requirement]; renamed {
		requirement = previous
	}
	identity := base[baseKey(requirement, slot.scenario)]
	if identity == "" || registry.change[identity] {
		return "", "generated"
	}
	return identity, "base"
}
