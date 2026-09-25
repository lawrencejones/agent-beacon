package learning

import (
	"fmt"
	"strings"

	"github.com/asymptote-labs/agent-beacon/pkg/asymptoteobserve"
)

// MemoryKinds lists the kinds a reviewer may assign to a memory candidate.
var MemoryKinds = []string{
	asymptoteobserve.LearningMemoryKindWorkflow,
	asymptoteobserve.LearningMemoryKindCorrection,
	asymptoteobserve.LearningMemoryKindDebuggingPattern,
	asymptoteobserve.LearningMemoryKindGotcha,
	asymptoteobserve.LearningMemoryKindConvention,
}

// CandidateContent is reviewer-written memory content. An empty field means "not
// provided": CandidateFromTrace requires a kind, title and body, while
// ApproveCandidateWithContent applies only the fields that are set.
type CandidateContent struct {
	Kind          string
	Title         string
	Body          string
	Applicability string
	Tags          []string
}

// IsZero reports whether no content was provided at all.
func (c CandidateContent) IsZero() bool {
	return strings.TrimSpace(c.Kind) == "" && strings.TrimSpace(c.Title) == "" &&
		strings.TrimSpace(c.Body) == "" && strings.TrimSpace(c.Applicability) == "" && len(c.Tags) == 0
}

func (c CandidateContent) validateKind() error {
	kind := strings.TrimSpace(c.Kind)
	if kind == "" {
		return nil
	}
	for _, known := range MemoryKinds {
		if kind == known {
			return nil
		}
	}
	return fmt.Errorf("unknown memory kind %q; expected one of %s", kind, strings.Join(MemoryKinds, ", "))
}

// CandidateFromTrace builds a review candidate from a trace and content written by
// whoever read it. No evaluator is involved: the reviewer states the lesson, and the
// candidate records the trace and its event IDs as evidence. SourceEvaluationID stays
// empty, which is how an authored candidate is told apart from a Jev-derived one.
func CandidateFromTrace(project asymptoteobserve.LearningProjectV1, trace asymptoteobserve.TraceShowResultV1, content CandidateContent) (asymptoteobserve.LearningCandidateV1, error) {
	if strings.TrimSpace(trace.Trace.ID) == "" {
		return asymptoteobserve.LearningCandidateV1{}, fmt.Errorf("trace id is required")
	}
	kind := strings.TrimSpace(content.Kind)
	title := strings.TrimSpace(content.Title)
	body := strings.TrimRight(content.Body, " \t\r\n")
	if kind == "" {
		return asymptoteobserve.LearningCandidateV1{}, fmt.Errorf("kind is required; expected one of %s", strings.Join(MemoryKinds, ", "))
	}
	if err := content.validateKind(); err != nil {
		return asymptoteobserve.LearningCandidateV1{}, err
	}
	if title == "" {
		return asymptoteobserve.LearningCandidateV1{}, fmt.Errorf("title is required")
	}
	if strings.TrimSpace(body) == "" {
		return asymptoteobserve.LearningCandidateV1{}, fmt.Errorf("body is required")
	}
	applicability := strings.TrimSpace(content.Applicability)
	if applicability == "" {
		applicability = "When a future agent in this project hits a similar workflow, regardless of harness."
	}
	tags := cleanTags(content.Tags)
	if len(tags) == 0 {
		tags = []string{"beacon", kind}
		if trace.Trace.Harness.Name != "" {
			tags = append(tags, trace.Trace.Harness.Name)
		}
	}
	evidence := asymptoteobserve.LearningEvidenceV1{
		TraceID: strings.TrimSpace(trace.Trace.ID),
		Summary: strings.TrimSpace(trace.Trace.Title),
	}
	for _, event := range trace.Events {
		evidence.EventIDs = append(evidence.EventIDs, event.ID)
	}
	candidate := asymptoteobserve.LearningCandidateV1{
		SchemaVersion: asymptoteobserve.LearningSchemaVersion,
		State:         asymptoteobserve.LearningCandidateStateCandidate,
		Kind:          kind,
		Title:         title,
		Body:          body,
		Applicability: applicability,
		Tags:          tags,
		Project:       project,
		Evidence:      []asymptoteobserve.LearningEvidenceV1{evidence},
	}
	candidate.ID = CandidateID(candidate)
	return candidate, nil
}

// applyContent overwrites the candidate fields that content sets and leaves the rest.
func applyContent(candidate asymptoteobserve.LearningCandidateV1, content CandidateContent) (asymptoteobserve.LearningCandidateV1, error) {
	if err := content.validateKind(); err != nil {
		return candidate, err
	}
	if kind := strings.TrimSpace(content.Kind); kind != "" {
		candidate.Kind = kind
	}
	if title := strings.TrimSpace(content.Title); title != "" {
		candidate.Title = title
	}
	if body := strings.TrimRight(content.Body, " \t\r\n"); strings.TrimSpace(body) != "" {
		candidate.Body = body
	}
	if applicability := strings.TrimSpace(content.Applicability); applicability != "" {
		candidate.Applicability = applicability
	}
	if tags := cleanTags(content.Tags); len(tags) > 0 {
		candidate.Tags = tags
	}
	return candidate, nil
}

func cleanTags(tags []string) []string {
	var out []string
	seen := map[string]bool{}
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if tag == "" || seen[tag] {
			continue
		}
		seen[tag] = true
		out = append(out, tag)
	}
	return out
}
