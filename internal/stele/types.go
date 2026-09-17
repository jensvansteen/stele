package stele

// Source identifies a location in a repository input.
type Source struct {
	Path string `json:"path"`
	Line int    `json:"line"`
}

// Diagnostic describes a deterministic validation finding.
type Diagnostic struct {
	Code       string  `json:"code"`
	Severity   string  `json:"severity"`
	Message    string  `json:"message"`
	IdentityID *string `json:"identityId"`
	Source     *Source `json:"source"`
}

// Scenario is a parsed OpenSpec behavior scenario.
type Scenario struct {
	ID     string
	Title  string
	Source Source
	// Text is the scenario block without its Verification-ID line.
	Text string
}

// Requirement is a parsed OpenSpec requirement and its scenarios.
type Requirement struct {
	ID        string
	Title     string
	Source    Source
	Scenarios []Scenario
}

// ParsedSpecs contains the canonical behaviors and diagnostics read from OpenSpec.
type ParsedSpecs struct {
	Requirements []Requirement
	Diagnostics  []Diagnostic
	Files        []string
}

// Anchor is a source or test declaration linked to a verification identity.
type Anchor struct {
	ID              string  `json:"id"`
	Annotation      string  `json:"annotation"`
	Kind            string  `json:"kind"`
	Path            string  `json:"path"`
	Line            int     `json:"line"`
	Selector        *string `json:"selector"`
	DeclarationLine *int    `json:"declarationLine"`
	// EvidenceID is the full level-qualified ID of a `@verifies` anchor, such as
	// scn.todo.0a1b2c3d4e5f.unit, and empty for a bare identity.
	EvidenceID string `json:"evidenceId,omitempty"`
	Level      string `json:"level,omitempty"`
}

// EvidenceApproval records who approved an evidence entry and what they saw.
type EvidenceApproval struct {
	Approver string `json:"approver"`
	Date     string `json:"date"`
	Digest   string `json:"digest"`
	Via      string `json:"via"`
	Revision string `json:"revision,omitempty"`
}

// EvidenceEntry is one planned kind of evidence for a scenario.
type EvidenceEntry struct {
	ID        string            `json:"id"`
	Level     string            `json:"level"`
	Rationale string            `json:"rationale"`
	Placement string            `json:"placement,omitempty"`
	Approval  *EvidenceApproval `json:"approval,omitempty"`
}

// ScenarioEvidence lists the planned evidence of one scenario in a v2 plan.
type ScenarioEvidence struct {
	Evidence []EvidenceEntry `json:"evidence"`
}

// LinkagePlan records the planned links of a change: v1 code and test targets,
// or v2 evidence entries per scenario.
type LinkagePlan struct {
	SchemaVersion int               `json:"schemaVersion"`
	ChangeID      string            `json:"changeId"`
	Requirements  map[string]string `json:"requirements"`
	Scenarios     map[string]string `json:"scenarios"`
	// Evidence holds v2 entries by scenario. EvidenceOnly is set for a v2 change
	// plan, where every scenario follows the v2 rules.
	Evidence     map[string][]EvidenceEntry `json:"-"`
	EvidenceOnly bool                       `json:"-"`
	// Source is the repository path the plan was read from.
	Source string `json:"-"`
}

// PlannedEvidence is the report summary of one evidence entry.
type PlannedEvidence struct {
	ID        string `json:"id"`
	Level     string `json:"level"`
	Approval  string `json:"approval"`
	Placement string `json:"placement,omitempty"`
}

// Link describes a resolved or planned relationship between an identity and a repository target.
type Link struct {
	ID              string  `json:"id,omitempty"`
	Annotation      string  `json:"annotation,omitempty"`
	Kind            string  `json:"kind"`
	Path            string  `json:"path,omitempty"`
	Line            int     `json:"line,omitempty"`
	Selector        *string `json:"selector,omitempty"`
	DeclarationLine *int    `json:"declarationLine,omitempty"`
	State           string  `json:"state"`
	Target          *string `json:"target,omitempty"`
	EvidenceID      string  `json:"evidenceId,omitempty"`
	Level           string  `json:"level,omitempty"`
}

// ExecutionState separates test execution state from its outcome.
type ExecutionState struct {
	State   string `json:"state"`
	Outcome string `json:"outcome"`
}

// ScenarioReport is the verification result for one canonical scenario.
type ScenarioReport struct {
	ID        string         `json:"id"`
	Title     string         `json:"title"`
	Source    Source         `json:"source"`
	TestLinks []Link         `json:"testLinks"`
	Linkage   string         `json:"linkage"`
	Execution ExecutionState `json:"execution"`
	// Evidence lists the planned evidence of a scenario under a v2 plan.
	Evidence []PlannedEvidence `json:"evidence,omitempty"`
}

// RequirementReport is the verification result for one canonical requirement.
type RequirementReport struct {
	ID        string           `json:"id"`
	Title     string           `json:"title"`
	Status    string           `json:"status"`
	Source    Source           `json:"source"`
	CodeLinks []Link           `json:"codeLinks"`
	Linkage   string           `json:"linkage"`
	Scenarios []ScenarioReport `json:"scenarios"`
}

// VerifierMetadata identifies the producer of a verification report.
type VerifierMetadata struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// OpenSpecMetadata identifies the OpenSpec input represented by a report.
type OpenSpecMetadata struct {
	Version  string `json:"version"`
	ChangeID string `json:"changeId"`
}

// RepositoryMetadata captures the repository state covered by verification.
type RepositoryMetadata struct {
	Revision    string `json:"revision"`
	Dirty       bool   `json:"dirty"`
	InputDigest string `json:"inputDigest"`
}

// StageStatus is the result of a verification stage without revision metadata.
type StageStatus struct {
	Status string `json:"status"`
}

// ExecutionStage is the test execution state included in a report.
type ExecutionStage struct {
	Status         string  `json:"status"`
	TestedRevision *string `json:"testedRevision"`
}

// ReviewStage is the human review state included in a report.
type ReviewStage struct {
	Status           string  `json:"status"`
	ReviewedRevision *string `json:"reviewedRevision"`
}

// VerificationStages keeps proposal, linkage, execution, and review states distinct.
type VerificationStages struct {
	Proposal  StageStatus    `json:"proposal"`
	Linkage   StageStatus    `json:"linkage"`
	Execution ExecutionStage `json:"execution"`
	Review    ReviewStage    `json:"review"`
}

// ReportSummary contains aggregate counts for a verification report.
type ReportSummary struct {
	Requirements       int `json:"requirements"`
	Scenarios          int `json:"scenarios"`
	LinkedRequirements int `json:"linkedRequirements"`
	LinkedScenarios    int `json:"linkedScenarios"`
	PassedScenarios    int `json:"passedScenarios"`
	Errors             int `json:"errors"`
	Warnings           int `json:"warnings"`
}

// Report is the deterministic verification result for one OpenSpec change.
type Report struct {
	SchemaVersion string              `json:"schemaVersion"`
	Verifier      VerifierMetadata    `json:"verifier"`
	OpenSpec      OpenSpecMetadata    `json:"openspec"`
	Mode          string              `json:"mode"`
	Repository    RepositoryMetadata  `json:"repository"`
	Complete      bool                `json:"complete"`
	Verdict       string              `json:"verdict"`
	Stages        VerificationStages  `json:"stages"`
	Summary       ReportSummary       `json:"summary"`
	Requirements  []RequirementReport `json:"requirements"`
	Diagnostics   []Diagnostic        `json:"diagnostics"`
}

// ScenarioOutcome records the execution outcome associated with a scenario identity.
type ScenarioOutcome struct {
	ID      string `json:"id"`
	Outcome string `json:"outcome"`
	// FailedEvidence names the evidence IDs whose tests did not pass.
	FailedEvidence []string `json:"failedEvidence,omitempty"`
}

// TestExecution records one exact test invocation and the scenarios it covers.
type TestExecution struct {
	Path        string   `json:"path"`
	Selector    *string  `json:"selector"`
	ScenarioIDs []string `json:"scenarioIds"`
	EvidenceIDs []string `json:"evidenceIds,omitempty"`
	Outcome     string   `json:"outcome"`
	Reason      *string  `json:"reason"`
}

// Evidence is the deterministic execution manifest consumed by verification.
type Evidence struct {
	SchemaVersion  int               `json:"schemaVersion"`
	Runner         string            `json:"runner"`
	TestedRevision string            `json:"testedRevision"`
	InputDigest    string            `json:"inputDigest"`
	Outcome        string            `json:"outcome"`
	Scenarios      []ScenarioOutcome `json:"scenarios"`
	Executions     []TestExecution   `json:"executions"`
}

// IdentityInsertion is one Verification-ID that was inserted or, when checking, is missing.
type IdentityInsertion struct {
	ID     string `json:"id"`
	Kind   string `json:"kind"`
	Title  string `json:"title"`
	Path   string `json:"path"`
	Line   int    `json:"line"`
	Origin string `json:"origin"`
}

// IdentityResult reports the Verification-IDs assigned to a change's delta specs.
type IdentityResult struct {
	SchemaVersion int                 `json:"schemaVersion"`
	ChangeID      string              `json:"changeId"`
	Mode          string              `json:"mode"`
	Verdict       string              `json:"verdict"`
	Insertions    []IdentityInsertion `json:"insertions"`
}

// Config selects the adapter and default OpenSpec change for a project.
type Config struct {
	SchemaVersion int    `json:"schemaVersion"`
	Adapter       string `json:"adapter"`
	Change        string `json:"change"`
}
