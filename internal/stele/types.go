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
}

// LinkagePlan records the implementation and test targets planned for a change.
type LinkagePlan struct {
	SchemaVersion int               `json:"schemaVersion"`
	ChangeID      string            `json:"changeId"`
	Requirements  map[string]string `json:"requirements"`
	Scenarios     map[string]string `json:"scenarios"`
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
}

// TestExecution records one exact test invocation and the scenarios it covers.
type TestExecution struct {
	Path        string   `json:"path"`
	Selector    *string  `json:"selector"`
	ScenarioIDs []string `json:"scenarioIds"`
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

// Config selects the adapter and default OpenSpec change for a project.
type Config struct {
	SchemaVersion int    `json:"schemaVersion"`
	Adapter       string `json:"adapter"`
	Change        string `json:"change"`
}
