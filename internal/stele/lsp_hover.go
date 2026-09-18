package stele

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
)

// lspCard renders the lines of a hover card in Markdown or in plain text.
type lspCard struct {
	markdown bool
	lines    []string
}

func (card *lspCard) add(lines ...string) {
	card.lines = append(card.lines, lines...)
}

func (card *lspCard) bold(text string) string {
	return choose(card.markdown, "**"+text+"**", text)
}

func (card *lspCard) code(text string) string {
	return choose(card.markdown, "`"+text+"`", text)
}

// rule separates the parts of a card.
func (card *lspCard) rule() {
	card.add("", choose(card.markdown, "---", "—"), "")
}

// item is one list item.
func (card *lspCard) item(text string) string {
	return "- " + text
}

func (card *lspCard) String() string {
	return strings.Join(card.lines, "\n")
}

// lspHoverText renders the hover of a target: on an anchor, a card per scope
// that declares its ID, with the evidence's status for an evidence anchor; on
// a heading, the card of that scope and where the behavior is implemented and
// tested.
//
// @implements req.languageserver.c8c10808862b
func lspHoverText(index Index, target lspTarget, markdown bool) string {
	card := &lspCard{markdown: markdown}
	first := true
	for _, requirement := range index.Requirements {
		if requirement.ID != target.id || (target.heading() && requirement.Scope != target.scope) {
			continue
		}
		if !first {
			card.rule()
		}
		first = false
		requirementCard(card, index, requirement, target.heading())
	}
	for _, scenario := range index.Scenarios {
		if scenario.ID != target.id || (target.heading() && scenario.Scope != target.scope) {
			continue
		}
		if !first {
			card.rule()
		}
		first = false
		scenarioCard(card, scenario)
		switch {
		case target.heading():
			card.rule()
			card.add(card.bold("Tests"))
			evidenceLines(card, scenario, "")
		case target.evidence != "" || !isRequirementID(target.id):
			evidenceStatusLines(card, scenario, choose(target.evidence == "", target.id, target.evidence))
		}
	}
	return card.String()
}

// requirementCard adds a requirement's title, scope, and text, and on its
// heading where it is implemented and tested.
func requirementCard(card *lspCard, index Index, requirement IndexRequirement, heading bool) {
	card.add(card.bold("Requirement:")+" "+requirement.Title+" · "+card.code(requirement.Scope), "",
		requirement.Text)
	if !heading {
		return
	}
	card.rule()
	card.add(card.bold("Implementations"))
	if len(requirement.Implementations) == 0 {
		card.add("none yet")
	}
	for _, location := range requirement.Implementations {
		card.add(card.item(locationText(card, location)))
	}
	card.add("", card.bold("Tests"))
	for _, scenario := range index.Scenarios {
		if scenario.Requirement == requirement.ID && scenario.Scope == requirement.Scope {
			evidenceLines(card, scenario, scenario.Title+": ")
		}
	}
}

// scenarioCard adds a scenario's title, scope, and steps.
func scenarioCard(card *lspCard, scenario IndexScenario) {
	card.add(card.bold("Scenario:")+" "+scenario.Title+" · "+card.code(scenario.Scope), "")
	if len(scenario.Steps) == 0 {
		card.add(scenario.Text)
		return
	}
	for _, step := range scenario.Steps {
		line := step.Text
		if step.Keyword != "" {
			line = card.bold(step.Keyword) + " " + step.Text
		}
		card.add(choose(card.markdown, card.item(line), line))
	}
}

// evidenceStatusLines adds the status of the evidence entries an anchor names.
func evidenceStatusLines(card *lspCard, scenario IndexScenario, evidenceID string) {
	for _, evidence := range scenario.Evidence {
		if evidence.ID == evidenceID {
			card.rule()
			card.add(evidenceStatus(card, evidence))
		}
	}
}

// evidenceLines lists a scenario's evidence with its tests, or says it has none.
func evidenceLines(card *lspCard, scenario IndexScenario, prefix string) {
	if len(scenario.Evidence) == 0 {
		card.add(card.item(prefix + "no planned evidence or tests"))
	}
	for _, evidence := range scenario.Evidence {
		line := prefix + evidenceStatus(card, evidence)
		if len(evidence.Tests) == 0 {
			line = prefix + evidenceLevel(card, evidence) + " · " + approvalLabel(evidence.Approval) +
				" · planned, no test"
		}
		locations := make([]string, 0, len(evidence.Tests))
		for _, test := range evidence.Tests {
			locations = append(locations, locationText(card, test))
		}
		if len(locations) > 0 {
			line += " — " + strings.Join(locations, ", ")
		}
		card.add(card.item(line))
	}
}

// evidenceStatus is an evidence entry's level, approval, and last outcome.
func evidenceStatus(card *lspCard, evidence IndexEvidence) string {
	outcome := strings.ReplaceAll(evidence.Execution.Outcome, "-", " ")
	if evidence.Execution.State == "stale" {
		outcome += " (stale)"
	}
	return evidenceLevel(card, evidence) + " · " + approvalLabel(evidence.Approval) + " · " + outcome
}

func evidenceLevel(card *lspCard, evidence IndexEvidence) string {
	return card.code(choose(evidence.Level == "", "tests", evidence.Level))
}

// approvalText describes an evidence entry's approval state.
func approvalLabel(approval string) string {
	switch approval {
	case "stale":
		return "approval stale"
	case "v1":
		return "v1 plan"
	default:
		return approval
	}
}

func locationText(card *lspCard, location IndexLocation) string {
	text := card.code(fmt.Sprintf("%s:%d", location.Path, location.Line))
	if location.Selector != nil {
		text += " " + *location.Selector
	}
	return text
}

// hover answers textDocument/hover with the card of the target under the
// cursor, in Markdown when the client declares it and plain text otherwise.
func (server *lspServer) hover(raw json.RawMessage) ([]byte, error) {
	var params lspTextDocumentPositionParams
	if err := json.Unmarshal(raw, &params); err != nil {
		return nil, err
	}
	project, relative, lines, found := server.document(params.TextDocument.URI)
	if !found {
		return []byte("null"), nil
	}
	target, found := lspTargetAt(project.build.index, relative, lines, params.Position, server.encoding)
	if !found {
		return []byte("null"), nil
	}
	markdown := slices.Contains(server.client.TextDocument.Hover.ContentFormat, "markdown")
	text := lspHoverText(project.build.index, target, markdown)
	if text == "" {
		return []byte("null"), nil
	}
	return json.Marshal(lspHover{
		Contents: lspMarkupContent{Kind: choose(markdown, "markdown", "plaintext"), Value: text},
		Range:    target.span,
	})
}
