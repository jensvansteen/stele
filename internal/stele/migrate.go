package stele

import (
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"slices"
	"sort"
	"strings"
)

const (
	migratedRationale        = "Migrated from v1; review level and rationale."
	migratedWithoutLevelNote = "Migrated from v1 without a level; choose the level."
)

var scanMigrationAnchors = ScanAnchors

type migrationResult struct {
	Path      string
	Scenarios int
	Entries   int
}

// migrateV1Plan converts the v1 plans of a scope to v2. Each scenario gets one
// unapproved entry per level its test anchors already name, or one unapproved
// unit entry flagged for review. Targets are dropped.
//
// @implements req.verificationstrategy.ef0c2440e601
func migrateV1Plan(root string, scope verificationScope) ([]migrationResult, error) {
	anchors, err := scanMigrationAnchors(root)
	if err != nil {
		return nil, err
	}
	paths := scope.spec().PlanPaths(root, scope)
	results := make([]migrationResult, 0, len(paths))
	for _, path := range paths {
		plan, err := readLinkagePlan(path)
		if err != nil {
			return results, fmt.Errorf("read %s: %w", filepath.ToSlash(path), err)
		}
		if plan.SchemaVersion == evidencePlanVersion {
			continue
		}
		converted := convertV1Plan(plan, anchors)
		if err := writePlanFile(path, converted); err != nil {
			return results, err
		}
		result := migrationResult{Path: repositoryPath(root, path), Scenarios: len(converted.Scenarios)}
		for _, scenario := range converted.Scenarios {
			result.Entries += len(scenario.Evidence)
		}
		results = append(results, result)
	}
	return results, nil
}

func convertV1Plan(plan LinkagePlan, anchors []Anchor) evidencePlanFile {
	converted := evidencePlanFile{
		SchemaVersion: evidencePlanVersion,
		ChangeID:      plan.ChangeID,
		Scenarios:     map[string]ScenarioEvidence{},
	}
	for id := range plan.Scenarios {
		levels := make([]string, 0)
		for _, anchor := range anchorsFor(anchors, id, "test") {
			if anchor.EvidenceID != "" && !slices.Contains(levels, anchor.EvidenceID) {
				levels = append(levels, anchor.EvidenceID)
			}
		}
		sort.Slice(levels, func(i, j int) bool { return evidenceOrder(id, levels[i]) < evidenceOrder(id, levels[j]) })
		entries := make([]EvidenceEntry, 0, max(len(levels), 1))
		for _, evidence := range levels {
			level, _, _ := strings.Cut(strings.TrimPrefix(evidence, id+"."), ".")
			entries = append(entries, EvidenceEntry{ID: evidence, Level: level, Rationale: migratedRationale})
		}
		if len(entries) == 0 {
			entries = append(entries, EvidenceEntry{
				ID:        evidenceID(id, "unit"),
				Level:     "unit",
				Rationale: migratedWithoutLevelNote,
			})
		}
		converted.Scenarios[id] = ScenarioEvidence{Evidence: entries}
	}
	return converted
}

// evidenceOrder sorts anchored evidence IDs, whose levels are always valid, by
// level (unit, integration, e2e) and then by ordinal.
func evidenceOrder(scenarioID, id string) string {
	level, ordinal, _ := splitEvidenceID(scenarioID, id)
	rank := slices.Index(evidenceLevels, level)
	return fmt.Sprintf("%d:%09d:%s", rank, ordinal, id)
}

func migrateCommand(parsed options, stdout, stderr io.Writer) int {
	results, err := migrateV1Plan(parsed.root, resolveScope(parsed))
	if err != nil {
		return writeCommandError(stderr, err)
	}
	if len(results) == 0 {
		_, _ = fmt.Fprintln(stdout, "No v1 linkage plans to migrate.")
		return 0
	}
	for _, result := range results {
		_, _ = fmt.Fprintf(stdout, "✓ migrated %s: %d scenarios, %d unapproved evidence entries\n",
			result.Path, result.Scenarios, result.Entries)
	}
	_, _ = fmt.Fprintln(stdout, "Review the levels and rationales, then approve them with stele approve.")
	return 0
}

var errUnknownPlanCommand = errors.New("unknown plan command; use: stele plan migrate")
