package stele

import "strings"

// The stages a diagnostic belongs to, as the human report names them.
const (
	stageSpecifications = "Specifications"
	stagePlan           = "Plan approval"
	stageLinkage        = "Linkage (anchors)"
	// stageExecution holds the execution codes, which only the language
	// server reports; the CLI reports execution in its verdicts.
	stageExecution = "Test execution"
)

// diagnosticGuide explains one diagnostic code: the stage it fails, a
// one-line meaning, a short label for the verdict line, and a fix step.
// "{scope}" in the fix becomes the scope's flag, such as --specs.
type diagnosticGuide struct {
	stage   string
	meaning string
	short   string
	fix     string
}

// diagnosticGuides is the catalogue of every diagnostic code Stele reports.
var diagnosticGuides = map[string]diagnosticGuide{
	"ID_REQUIREMENT_MISSING": {
		stageSpecifications, "A requirement has no Verification-ID.", "missing ID{s}",
		"run `stele ids {scope}` to insert the missing IDs",
	},
	"ID_SCENARIO_MISSING": {
		stageSpecifications, "A scenario has no Verification-ID.", "missing ID{s}",
		"run `stele ids {scope}` to insert the missing IDs",
	},
	"ID_FORMAT": {
		stageSpecifications, "A Verification-ID is malformed or of the wrong kind.", "malformed ID{s}",
		"remove the line and run `stele ids {scope}`; never write an ID by hand",
	},
	"ID_MULTIPLE": {
		stageSpecifications, "A heading has more than one Verification-ID.", "repeated ID{s}",
		"keep one `Verification-ID:` line under the heading",
	},
	"ID_DUPLICATE": {
		stageSpecifications, "Two headings declare the same Verification-ID.", "duplicate ID{s}",
		"remove the copied line and run `stele ids {scope}` for the new heading",
	},
	"SCENARIO_MISSING": {
		stageSpecifications, "A requirement has no scenarios.", "requirement{s} without scenarios",
		"add at least one `#### Scenario:` with WHEN and THEN steps",
	},
	"SPEC_ANNOTATION_MISSING": {
		stageSpecifications, "A specification file has no Stele annotation.",
		"unannotated file{s}", "run `stele annotate {scope}`",
	},
	"SPEC_ANNOTATION_MISPLACED": {
		stageSpecifications, "The Stele annotation is not on the first line.",
		"misplaced annotation{s}", "move `<!-- stele: spec v1 -->` to line 1",
	},
	"SPEC_ANNOTATION_MALFORMED": {
		stageSpecifications, "The first-line Stele annotation is malformed.",
		"malformed annotation{s}", "write the first line as `<!-- stele: spec v1 -->`",
	},
	"SPEC_ANNOTATION_UNSUPPORTED": {
		stageSpecifications, "The annotation declares an unsupported format version.",
		"unsupported annotation{s}", "upgrade Stele, or write `<!-- stele: spec v1 -->`",
	},
	"SPEC_ANNOTATION_FIELD_IGNORED": {
		stageSpecifications, "The annotation has a field that format v1 ignores.",
		"ignored field{s}", "remove the field from the annotation",
	},
	"SPEC_REMOVED_UNMATCHED": {
		stageSpecifications,
		"A removed requirement name matches no current requirement, so nothing is checked.", "unmatched removal{s}",
		"copy the exact name from openspec/specs/<capability>/spec.md, or remove the entry",
	},
	"PLAN_UNAPPROVED": {
		stagePlan, "Planned evidence has no human approval yet.", "unapproved",
		"review the levels in design.md, then run `stele approve {scope}` " +
			"(agents: after an explicit yes in chat, `stele approve {scope} --confirmed-in-chat`)",
	},
	"PLAN_APPROVAL_STALE": {
		stagePlan, "The scenario or the entry changed after it was approved.", "stale approval{s}",
		"review the change, then run `stele approve {scope}` again",
	},
	"PLAN_EVIDENCE_MISSING": {
		stagePlan, "A scenario has no planned evidence.", "unplanned scenario{s}",
		"add an evidence entry for the scenario to linkage-plan.json, as the stele-plan skill describes",
	},
	"PLAN_EVIDENCE_INVALID": {
		stagePlan, "An evidence entry is malformed.", "invalid entr{ies}",
		"fix the entry's ID, level, and rationale in linkage-plan.json",
	},
	"PLAN_UNKNOWN_ID": {
		stagePlan, "The plan names an ID the scope does not declare.", "unknown ID{s}",
		"remove the entry from linkage-plan.json, or fix its ID",
	},
	"PLAN_CHANGE_MISMATCH": {
		stagePlan, "The linkage plan belongs to another change.", "foreign plan{s}",
		"store the change's own plan in openspec/changes/<change>/linkage-plan.json",
	},
	"PLAN_CODE_MISSING": {
		stagePlan, "A version 1 plan has no code target for a requirement.", "missing target{s}",
		"migrate the plan with `stele plan migrate {scope}`",
	},
	"PLAN_TEST_MISSING": {
		stagePlan, "A version 1 plan has no test target for a scenario.", "missing target{s}",
		"migrate the plan with `stele plan migrate {scope}`",
	},
	"PLAN_V1_DEPRECATED": {
		stagePlan, "The linkage plan uses schema version 1, supported until 0.2.0.",
		"version 1 plan{s}", "run `stele plan migrate {scope}`, review the migrated levels, and approve them",
	},
	"PLAN_ARCHIVE_ORDER_AMBIGUOUS": {
		stagePlan,
		"Two changes archived on the same date plan an ID differently.", "ambiguous archive{s}",
		"approve the intended entries with `stele approve --specs`, or make the archived plans agree",
	},
	"PLAN_REMOVED_BEHAVIOR_PLANNED": {
		stagePlan, "The plan still lists evidence for removed behavior.",
		"removed-behavior entr{ies}", "delete the entry from linkage-plan.json, then run the proposal check again",
	},
	"LINK_CODE_MISSING": {
		stageLinkage, "No code has `@implements` for a requirement.", "unimplemented requirement{s}",
		"add `@implements <requirement-id>` above the implementing declaration",
	},
	"LINK_TEST_MISSING": {
		stageLinkage, "No test has `@verifies` for a scenario.", "untested scenario{s}",
		"add `@verifies <scenario-id>` above the test",
	},
	"LINK_EVIDENCE_MISSING": {
		stageLinkage, "No test has `@verifies` for an approved evidence entry.",
		"missing evidence test{s}", "add `@verifies <evidence-id>` above the test the plan's placement names",
	},
	"LINK_TARGET_MISMATCH": {
		stageLinkage, "An anchor does not resolve to the planned target.", "mismatched target{s}",
		"move the anchor to the planned declaration, or migrate the plan",
	},
	"LINK_REMOVED_BEHAVIOR_ANCHORED": {
		stageLinkage, "Code or a test is still anchored to removed behavior.",
		"removed-behavior anchor{s}",
		"delete the code or test, or move its anchor to the behavior it now serves",
	},
	"ANCHOR_DANGLING": {
		stageLinkage, "An anchor names an ID no specification declares.", "dangling anchor{s}",
		"fix the ID, or remove the anchor",
	},
	"ANCHOR_KIND": {
		stageLinkage, "An anchor has the wrong kind for its ID.", "wrong anchor kind{s}",
		"use `@implements` for req IDs and `@verifies` for scn and evidence IDs",
	},
	"ANCHOR_TARGET_MISSING": {
		stageLinkage, "An anchor is not directly above a declaration or named test.",
		"unattached anchor{s}", "put the anchor comment directly above the declaration or test with an exact name",
	},
	"EXECUTION_FAILED": {
		stageExecution, "The last run of an evidence entry's tests failed.", "failed test{s}",
		"fix the code or the test, then run `stele test <evidence-id> {scope}`",
	},
	"EXECUTION_STALE": {
		stageExecution, "The stored outcome was recorded before the verified inputs changed.", "stale result{s}",
		"run `stele test <evidence-id> {scope}` to record a current outcome",
	},
	"ANCHOR_EVIDENCE_UNPLANNED": {
		stageLinkage, "A test verifies evidence the plan does not list.",
		"unplanned evidence anchor{s}", "use a planned evidence ID, or add the entry to the plan and approve it",
	},
}

// label returns a guide's short label for a count: "{s}" becomes "s" and
// "{ies}" becomes "ies" for more than one finding.
func (guide diagnosticGuide) label(count int) string {
	if count == 1 {
		return strings.NewReplacer("{s}", "", "{ies}", "y").Replace(guide.short)
	}
	return strings.NewReplacer("{s}", "s", "{ies}", "ies").Replace(guide.short)
}

// guideFor returns a code's guide from the catalogue. An unknown code counts
// as linkage.
//
// @implements req.terminalreport.1183f56d16a2
func guideFor(code string) diagnosticGuide {
	if guide, known := diagnosticGuides[code]; known {
		return guide
	}
	return diagnosticGuide{
		stageLinkage, "Unknown diagnostic.", strings.ToLower(code),
		"see the diagnostics table in docs/reference/cli.md",
	}
}

// fixFor fills the scope flag into a guide's fix step.
func (guide diagnosticGuide) fixFor(scopeFlag string) string {
	return strings.ReplaceAll(guide.fix, "{scope}", scopeFlag)
}
