package stele

import (
	"slices"
	"testing"
)

// fixtureMatrix computes the matrix of the targeted fixture's change with
// the given stored executions.
func fixtureMatrix(t *testing.T, root string, executions ...TestExecution) *TargetMatrix {
	t.Helper()
	scope := configuredScope(t, root, changeScope("example"))
	parsed, err := parseScopeSpecs(root, scope)
	if err != nil {
		t.Fatal(err)
	}
	plan, _ := loadScopePlan(root, scope)
	anchors, err := ScanAnchors(root)
	if err != nil {
		t.Fatal(err)
	}
	return buildTargetMatrix(matrixInput{
		parsed: parsed, plan: plan, anchors: anchors, executions: indexExecutions(&Evidence{Executions: executions}),
		inputDigest: "current", targets: scope.targets,
	})
}

func matrixRow(t *testing.T, matrix *TargetMatrix, scenario string) TargetMatrixRow {
	t.Helper()
	if matrix == nil {
		t.Fatal("no matrix")
	}
	for _, row := range matrix.Rows {
		if row.Scenario == scenario {
			return row
		}
	}
	t.Fatalf("no matrix row for %s in %#v", scenario, matrix.Rows)
	return TargetMatrixRow{}
}

func cellOf(t *testing.T, row TargetMatrixRow, target string) TargetMatrixCell {
	t.Helper()
	for _, cell := range row.Cells {
		if cell.Target == target {
			return cell
		}
	}
	t.Fatalf("no %s cell in %#v", target, row)
	return TargetMatrixCell{}
}

// @verifies scn.verificationtargets.e03ff09a1cd4.unit
func TestMatrixShowsGapsBetweenReplicas(t *testing.T) {
	root := targetedFixture(t)
	writeTargetedPlan(t, root, true,
		targetedEntry(shareTextID, "ios", "unit"), targetedEntry(shareSheetID, "ios", "unit"))
	writeTargetedTest(t, root, "ios/share.test.mts", shareTextID+".ios.unit", "ios share text")
	matrix := fixtureMatrix(t, root,
		execution("ios/share.test.mts", "ios share text", shareTextID+".ios.unit", "passed", "current"))
	if !slices.Equal(matrix.Targets, []string{"android", "ios"}) {
		t.Fatalf("matrix columns = %v", matrix.Targets)
	}
	row := matrixRow(t, matrix, shareTextID)
	ios, android := cellOf(t, row, "ios"), cellOf(t, row, "android")
	if ios.State != "passed" || !slices.Equal(ios.Evidence, []string{shareTextID + ".ios.unit"}) {
		t.Fatalf("ios cell = %#v", ios)
	}
	if android.State != "missing" || len(android.Evidence) != 0 {
		t.Fatalf("android cell = %#v", android)
	}
	if row.Title != "Share text contains every item" || row.Capability != "share" ||
		row.Requirement != shareRequirementID {
		t.Fatalf("row = %#v", row)
	}
}

// @verifies scn.verificationtargets.250dbc097c43.unit
func TestMatrixMarksScenariosThatDoNotApply(t *testing.T) {
	root := fixtureRoot(t)
	writeFixture(t, root, "stele.config.json", `{"change":"example","targets":{"api":{"paths":["api/**"]},`+
		`"web":{"paths":["web/**"]}}}`)
	writeFixture(t, root, "openspec/changes/example/specs/checkout/spec.md", `<!-- stele: spec v1; targets: api, web -->
### Requirement: Show the promo discount
Verification-ID: req.checkout.111111111111
#### Scenario: Show the discount line
Verification-ID: scn.checkout.222222222222
Targets: web
- **WHEN** a code is accepted
- **THEN** the page shows the discount
`)
	row := matrixRow(t, fixtureMatrix(t, root), "scn.checkout.222222222222")
	if cell := cellOf(t, row, "api"); cell.State != matrixNotApplicable || len(cell.Evidence) != 0 {
		t.Fatalf("api cell = %#v", cell)
	}
	if cell := cellOf(t, row, "web"); cell.State != "missing" {
		t.Fatalf("web cell = %#v", cell)
	}
	if fixtureMatrix(t, evidenceFixture(t)) != nil {
		t.Fatal("a scope without targeted specifications has a matrix")
	}
}

// @verifies scn.verificationtargets.35a9719d9446.unit
func TestMatrixPrefersAFailureOverOtherStates(t *testing.T) {
	root := targetedFixture(t)
	writeTargetedPlan(t, root, true,
		targetedEntry(shareTextID, "ios", "unit"), targetedEntry(shareTextID, "ios", "e2e"))
	writeTargetedTest(t, root, "ios/share.test.mts", shareTextID+".ios.unit", "ios share text")
	matrix := fixtureMatrix(t, root,
		execution("ios/share.test.mts", "ios share text", shareTextID+".ios.unit", "failed", "current"))
	cell := cellOf(t, matrixRow(t, matrix, shareTextID), "ios")
	if cell.State != "failed" ||
		!slices.Equal(cell.Evidence, []string{shareTextID + ".ios.unit", shareTextID + ".ios.e2e"}) {
		t.Fatalf("cell = %#v", cell)
	}
	for _, states := range [][]string{
		{"passed", "missing", "missing"},
		{"not-run", "stale", "stale"},
		{"unapproved", "missing", "missing"},
		{"passed", "not-run", "not-run"},
		{"stale", "unapproved", "unapproved"},
	} {
		first, second := slices.Index(matrixStateOrder, states[0]), slices.Index(matrixStateOrder, states[1])
		if matrixStateOrder[min(first, second)] != states[2] {
			t.Fatalf("%s and %s did not give %s", states[0], states[1], states[2])
		}
	}
}
