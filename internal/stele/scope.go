package stele

import (
	"errors"
	"fmt"
	"maps"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
)

const linkagePlanFile = "linkage-plan.json"

var errConflictingScope = errors.New("--specs cannot be combined with --change")

// verificationScope selects what a command verifies: one OpenSpec change, or the
// current specifications in openspec/specs.
type verificationScope struct {
	changeID     string
	currentSpecs bool
	backend      specificationBackend
	// unannotated is the unannotatedSpecs policy; empty means the default.
	unannotated string
}

func changeScope(changeID string) verificationScope {
	return verificationScope{changeID: changeID}
}

// files returns the repository files the scope's backend reads, the disk
// unless the backend reads through other files.
func (scope verificationScope) files() repoFiles {
	if backend, ok := scope.spec().(interface{ repo() repoFiles }); ok {
		return backend.repo()
	}
	return diskFiles{}
}

// spec returns the scope's specification backend, OpenSpec by default.
func (scope verificationScope) spec() specificationBackend {
	if scope.backend == nil {
		return specificationBackends[defaultAdapter]
	}
	return scope.backend
}

// resolveScope turns parsed options into the scope that verify, test, and
// validate run against.
//
// @implements req.verificationscope.bee89d6750ed
func resolveScope(parsed options) verificationScope {
	if parsed.specs {
		return verificationScope{currentSpecs: true, backend: parsed.backend, unannotated: parsed.unannotated}
	}
	return verificationScope{changeID: parsed.changeID, backend: parsed.backend, unannotated: parsed.unannotated}
}

// requireScopeSpecs rejects a scope without specification files, so a gate
// never passes after checking zero requirements.
//
// @implements req.verificationscope.270b822fff6b
func requireScopeSpecs(root string, scope verificationScope) error {
	if len(scope.spec().SpecFiles(root, scope)) > 0 {
		return nil
	}
	if scope.currentSpecs {
		return errors.New("no current specifications in openspec/specs")
	}
	return fmt.Errorf("change %s has no delta specs", scope.changeID)
}

func emptyLinkagePlan() LinkagePlan {
	return LinkagePlan{Requirements: map[string]string{}, Scenarios: map[string]string{}}
}

func loadScopePlan(root string, scope verificationScope) (LinkagePlan, []Diagnostic) {
	if scope.currentSpecs {
		return loadArchivedPlans(root, scope)
	}
	return loadChangePlan(root, scope)
}

// loadChangePlan reads the plan stored with the change, falling back to the
// shared artifacts plan, and rejects a plan written for a different change.
//
// @implements req.verificationscope.713ceb31377c
func loadChangePlan(root string, scope verificationScope) (LinkagePlan, []Diagnostic) {
	changeID := scope.changeID
	file := filepath.Join(root, "artifacts", linkagePlanFile)
	for _, candidate := range scope.spec().PlanPaths(root, scope) {
		if scope.files().isFile(candidate) {
			file = candidate
			break
		}
	}
	source := repositoryPath(root, file)
	plan, err := readLinkagePlanFrom(scope.files(), file)
	if err != nil {
		return emptyLinkagePlan(), nil
	}
	if plan.ChangeID != "" && plan.ChangeID != changeID {
		return emptyLinkagePlan(), []Diagnostic{diagnostic(
			"PLAN_CHANGE_MISMATCH",
			"error",
			fmt.Sprintf("Linkage plan %s belongs to change %s, not %s.", source, plan.ChangeID, changeID),
			source,
			1,
			"",
		)}
	}
	plan.Source = source
	if plan.SchemaVersion != evidencePlanVersion {
		return plan, []Diagnostic{deprecatedPlanDiagnostic(source)}
	}
	return plan, nil
}

// archivedPlan is the linkage plan of one archived change, with the text of
// every identity its archived delta specs declare.
type archivedPlan struct {
	plan LinkagePlan
	// name is the archive directory, such as 2026-09-18-link-index: its date
	// prefix and then the change name give the archive order.
	name  string
	texts map[string]string
}

// loadArchivedPlans combines the plans of archived changes. For each identity
// it uses the archived change whose delta spec declares the identity with the
// text the current specification has; when several or none do, the one
// archived last by date prefix and then name. OpenSpec records only the date,
// so when changes archived on the same date still compete with different
// entries, PLAN_ARCHIVE_ORDER_AMBIGUOUS names both plans.
//
// @implements req.verificationscope.bee89d6750ed
func loadArchivedPlans(root string, scope verificationScope) (LinkagePlan, []Diagnostic) {
	combined := emptyLinkagePlan()
	combined.Evidence = map[string][]EvidenceEntry{}
	diagnostics := make([]Diagnostic, 0)
	current := identityTexts(scope.files(), root, scope.spec().SpecFiles(root, scope))
	plans := make([]archivedPlan, 0)
	for _, file := range scope.spec().PlanPaths(root, scope) {
		plan, err := readLinkagePlanFrom(scope.files(), file)
		if err != nil {
			continue
		}
		plan.Source = repositoryPath(root, file)
		if plan.SchemaVersion != evidencePlanVersion {
			diagnostics = append(diagnostics, deprecatedPlanDiagnostic(plan.Source))
		}
		plans = append(plans, archivedPlan{
			plan:  plan,
			name:  filepath.Base(filepath.Dir(file)),
			texts: identityTexts(scope.files(), root, scope.spec().ArchivedSpecFiles(root, file)),
		})
	}
	sort.Slice(plans, func(i, j int) bool { return plans[i].name < plans[j].name })
	for _, id := range plannedIdentities(plans) {
		winner, runnerUp := archivedCandidates(plans, id, current[id])
		entry := winner.plan
		switch {
		case entry.Evidence[id] != nil:
			combined.Evidence[id] = entry.Evidence[id]
		case entry.Scenarios[id] != "":
			combined.Scenarios[id] = entry.Scenarios[id]
		default:
			combined.Requirements[id] = entry.Requirements[id]
		}
		if runnerUp != nil && archiveDate(runnerUp.name) == archiveDate(winner.name) &&
			plannedEntryKey(runnerUp.plan, id) != plannedEntryKey(entry, id) {
			diagnostics = append(diagnostics, diagnostic(
				"PLAN_ARCHIVE_ORDER_AMBIGUOUS",
				"warning",
				fmt.Sprintf("Archived plans %s and %s, archived on the same date, plan %s differently; "+
					"Stele uses %s.", runnerUp.plan.Source, entry.Source, id, entry.Source),
				entry.Source,
				1,
				id,
			))
		}
	}
	return combined, diagnostics
}

// archivedCandidates returns the archived plan that decides an identity and
// the plan that would decide it next. Plans whose archived text matches the
// current text come first; among them, or among all when none match, the
// plan archived last wins.
func archivedCandidates(plans []archivedPlan, id, text string) (archivedPlan, *archivedPlan) {
	all := make([]archivedPlan, 0)
	matching := make([]archivedPlan, 0)
	for _, candidate := range plans {
		if plannedEntryKey(candidate.plan, id) == "" {
			continue
		}
		all = append(all, candidate)
		if text != "" && candidate.texts[id] == text {
			matching = append(matching, candidate)
		}
	}
	if len(matching) > 0 {
		all = matching
	}
	winner := all[len(all)-1]
	if len(all) == 1 {
		return winner, nil
	}
	return winner, &all[len(all)-2]
}

// plannedIdentities lists every identity any archived plan names, sorted.
func plannedIdentities(plans []archivedPlan) []string {
	seen := make(map[string]bool)
	for _, archived := range plans {
		for id := range archived.plan.Evidence {
			seen[id] = true
		}
		for id := range archived.plan.Scenarios {
			seen[id] = true
		}
		for id := range archived.plan.Requirements {
			seen[id] = true
		}
	}
	identities := slices.Collect(maps.Keys(seen))
	sort.Strings(identities)
	return identities
}

// plannedEntryKey describes what a plan plans for an identity, or returns ""
// when the plan does not name it: evidence IDs and levels, or a v1 target.
func plannedEntryKey(plan LinkagePlan, id string) string {
	if entries, listed := plan.Evidence[id]; listed {
		parts := make([]string, 0, len(entries))
		for _, entry := range entries {
			parts = append(parts, entry.ID+"="+entry.Level)
		}
		return "evidence:" + strings.Join(parts, ",")
	}
	if target := plan.Scenarios[id]; target != "" {
		return "scenario:" + target
	}
	if target := plan.Requirements[id]; target != "" {
		return "requirement:" + target
	}
	return ""
}

// archiveDate returns the YYYY-MM-DD prefix of an archive directory name, or
// the whole name when it has none.
func archiveDate(name string) string {
	if archiveDatePattern.MatchString(name) {
		return name[:len("2006-01-02")]
	}
	return name
}

var archiveDatePattern = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}-`)

// identityTexts returns the whitespace-normalized text of every requirement
// and scenario the files declare, the same text approvals fingerprint.
func identityTexts(repo repoFiles, root string, files []string) map[string]string {
	parsed, _ := parseSpecFiles(repo, root, files, "")
	texts := make(map[string]string)
	for _, requirement := range parsed.Requirements {
		texts[requirement.ID] = strings.Join(strings.Fields(requirement.Title+" "+requirement.Text), " ")
		for _, scenario := range requirement.Scenarios {
			texts[scenario.ID] = normalizedScenarioText(scenario)
		}
	}
	delete(texts, "")
	return texts
}

// repositoryPath returns a file's slash-separated path relative to root.
func repositoryPath(root, file string) string {
	return strings.TrimPrefix(filepath.ToSlash(file), filepath.ToSlash(root)+"/")
}

// declaredIdentities returns every Verification-ID declared anywhere under
// openspec/: active changes, archived changes, and current specifications.
// It also returns the retired identities: those that only archived changes
// declare, which is behavior an archived change removed.
//
// @implements req.verificationscope.9c81618619df
func declaredIdentities(repo repoFiles, root string) (map[string]bool, map[string]bool, error) {
	declared := make(map[string]bool)
	live := make(map[string]bool)
	files := repo.walk(filepath.Join(root, "openspec"), func(path string) bool {
		return strings.EqualFold(filepath.Ext(path), ".md")
	})
	for _, file := range files {
		content, err := repo.readFile(file)
		if err != nil {
			return nil, nil, err
		}
		archived := strings.HasPrefix(repositoryPath(root, file), "openspec/changes/archive/")
		for line := range strings.SplitSeq(string(content), "\n") {
			if match := verificationIDPattern.FindStringSubmatch(line); match != nil {
				declared[match[1]] = true
				live[match[1]] = live[match[1]] || !archived
			}
		}
	}
	retired := make(map[string]bool)
	for id := range declared {
		if !live[id] {
			retired[id] = true
		}
	}
	return declared, retired, nil
}
