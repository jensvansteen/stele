package stele

type Source struct {
	Path string `json:"path"`
	Line int    `json:"line"`
}

type Diagnostic struct {
	Code       string  `json:"code"`
	Severity   string  `json:"severity"`
	Message    string  `json:"message"`
	IdentityID *string `json:"identityId"`
	Source     *Source `json:"source"`
}

type Scenario struct {
	ID     string
	Title  string
	Source Source
}

type Requirement struct {
	ID        string
	Title     string
	Source    Source
	Scenarios []Scenario
}

type ParsedSpecs struct {
	Requirements []Requirement
	Diagnostics  []Diagnostic
	Files        []string
}

type Anchor struct {
	ID              string  `json:"id"`
	Annotation      string  `json:"annotation"`
	Kind            string  `json:"kind"`
	Path            string  `json:"path"`
	Line            int     `json:"line"`
	Selector        *string `json:"selector"`
	DeclarationLine *int    `json:"declarationLine"`
}

type LinkagePlan struct {
	SchemaVersion int               `json:"schemaVersion"`
	ChangeID      string            `json:"changeId"`
	Requirements  map[string]string `json:"requirements"`
	Scenarios     map[string]string `json:"scenarios"`
}

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

type ExecutionState struct {
	State   string `json:"state"`
	Outcome string `json:"outcome"`
}

type ScenarioReport struct {
	ID        string         `json:"id"`
	Title     string         `json:"title"`
	Source    Source         `json:"source"`
	TestLinks []Link         `json:"testLinks"`
	Linkage   string         `json:"linkage"`
	Execution ExecutionState `json:"execution"`
}

type RequirementReport struct {
	ID        string           `json:"id"`
	Title     string           `json:"title"`
	Status    string           `json:"status"`
	Source    Source           `json:"source"`
	CodeLinks []Link           `json:"codeLinks"`
	Linkage   string           `json:"linkage"`
	Scenarios []ScenarioReport `json:"scenarios"`
}

type Report struct {
	SchemaVersion string `json:"schemaVersion"`
	Verifier      struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	} `json:"verifier"`
	OpenSpec struct {
		Version  string `json:"version"`
		ChangeID string `json:"changeId"`
	} `json:"openspec"`
	Mode       string `json:"mode"`
	Repository struct {
		Revision    string `json:"revision"`
		Dirty       bool   `json:"dirty"`
		InputDigest string `json:"inputDigest"`
	} `json:"repository"`
	Complete bool   `json:"complete"`
	Verdict  string `json:"verdict"`
	Stages   struct {
		Proposal struct {
			Status string `json:"status"`
		} `json:"proposal"`
		Linkage struct {
			Status string `json:"status"`
		} `json:"linkage"`
		Execution struct {
			Status         string  `json:"status"`
			TestedRevision *string `json:"testedRevision"`
		} `json:"execution"`
		Review struct {
			Status           string  `json:"status"`
			ReviewedRevision *string `json:"reviewedRevision"`
		} `json:"review"`
	} `json:"stages"`
	Summary struct {
		Requirements       int `json:"requirements"`
		Scenarios          int `json:"scenarios"`
		LinkedRequirements int `json:"linkedRequirements"`
		LinkedScenarios    int `json:"linkedScenarios"`
		PassedScenarios    int `json:"passedScenarios"`
		Errors             int `json:"errors"`
		Warnings           int `json:"warnings"`
	} `json:"summary"`
	Requirements []RequirementReport `json:"requirements"`
	Diagnostics  []Diagnostic        `json:"diagnostics"`
}

type ScenarioOutcome struct {
	ID      string `json:"id"`
	Outcome string `json:"outcome"`
}

type TestExecution struct {
	Path        string   `json:"path"`
	Selector    *string  `json:"selector"`
	ScenarioIDs []string `json:"scenarioIds"`
	Outcome     string   `json:"outcome"`
	Reason      *string  `json:"reason"`
}

type Evidence struct {
	SchemaVersion  int               `json:"schemaVersion"`
	Runner         string            `json:"runner"`
	TestedRevision string            `json:"testedRevision"`
	InputDigest    string            `json:"inputDigest"`
	Outcome        string            `json:"outcome"`
	Scenarios      []ScenarioOutcome `json:"scenarios"`
	Executions     []TestExecution   `json:"executions"`
}

type Config struct {
	SchemaVersion int    `json:"schemaVersion"`
	Adapter       string `json:"adapter"`
	Change        string `json:"change"`
}
