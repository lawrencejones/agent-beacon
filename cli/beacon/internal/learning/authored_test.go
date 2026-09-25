package learning

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/asymptote-labs/agent-beacon/pkg/asymptoteobserve"
)

func authoredTestTrace() asymptoteobserve.TraceShowResultV1 {
	return asymptoteobserve.TraceShowResultV1{
		Trace: asymptoteobserve.TraceSummaryV1{
			ID:      "session:claude_code:s1",
			Title:   "Fix the flaky smoke test",
			Harness: asymptoteobserve.TraceHarnessV1{Name: "claude_code"},
		},
		Events: []asymptoteobserve.TraceEventV1{{ID: "e1"}, {ID: "e2"}},
	}
}

func TestCandidateFromTraceRecordsEvidenceAndDefaults(t *testing.T) {
	project := asymptoteobserve.LearningProjectV1{ID: "project-1", Path: "/repo"}
	candidate, err := CandidateFromTrace(project, authoredTestTrace(), CandidateContent{
		Kind:  asymptoteobserve.LearningMemoryKindGotcha,
		Title: "  Smoke needs a warm cache  ",
		Body:  "Run smoke twice after a clean checkout.\n\n",
	})
	if err != nil {
		t.Fatalf("CandidateFromTrace: %v", err)
	}
	if candidate.ID == "" || !strings.HasPrefix(candidate.ID, "candidate_") {
		t.Fatalf("id = %q", candidate.ID)
	}
	if candidate.State != asymptoteobserve.LearningCandidateStateCandidate || candidate.SourceEvaluationID != "" {
		t.Fatalf("candidate = %#v", candidate)
	}
	if candidate.Title != "Smoke needs a warm cache" || candidate.Body != "Run smoke twice after a clean checkout." {
		t.Fatalf("title=%q body=%q", candidate.Title, candidate.Body)
	}
	if candidate.Project.ID != "project-1" {
		t.Fatalf("project = %#v", candidate.Project)
	}
	if len(candidate.Evidence) != 1 || candidate.Evidence[0].TraceID != "session:claude_code:s1" ||
		strings.Join(candidate.Evidence[0].EventIDs, ",") != "e1,e2" || candidate.Evidence[0].Summary != "Fix the flaky smoke test" {
		t.Fatalf("evidence = %#v", candidate.Evidence)
	}
	if strings.Join(candidate.Tags, ",") != "beacon,gotcha,claude_code" {
		t.Fatalf("tags = %#v", candidate.Tags)
	}
	if candidate.Applicability == "" {
		t.Fatal("applicability should default when not provided")
	}
}

func TestCandidateFromTraceValidates(t *testing.T) {
	project := asymptoteobserve.LearningProjectV1{ID: "project-1"}
	cases := map[string]CandidateContent{
		"missing kind":  {Title: "t", Body: "b"},
		"unknown kind":  {Kind: "anecdote", Title: "t", Body: "b"},
		"missing title": {Kind: asymptoteobserve.LearningMemoryKindWorkflow, Body: "b"},
		"missing body":  {Kind: asymptoteobserve.LearningMemoryKindWorkflow, Title: "t", Body: "  \n"},
	}
	for name, content := range cases {
		if _, err := CandidateFromTrace(project, authoredTestTrace(), content); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
	if _, err := CandidateFromTrace(project, asymptoteobserve.TraceShowResultV1{}, CandidateContent{Kind: "workflow", Title: "t", Body: "b"}); err == nil {
		t.Error("empty trace: expected an error")
	}
}

func TestApproveCandidateWithContentReplacesOnlyProvidedFields(t *testing.T) {
	store := Open(filepath.Join(t.TempDir(), "memory.db"))
	eval := asymptoteobserve.LearningEvaluationV1{
		SchemaVersion: asymptoteobserve.LearningSchemaVersion,
		ID:            "eval-1",
		Status:        asymptoteobserve.LearningEvaluationStatusCompleted,
		Score:         0.9,
		Project:       asymptoteobserve.LearningProjectV1{ID: "project-1"},
		Trace:         asymptoteobserve.LearningTraceRefV1{ID: "trace-1", Title: "ok, let's build this.", EventIDs: []string{"e1"}},
		Questions:     []asymptoteobserve.LearningEvaluationQuestionV1{{ID: "task_success", Probability: 0.9}},
	}
	candidate, ok := CandidateFromEvaluation(eval)
	if !ok {
		t.Fatal("expected a candidate")
	}
	if !strings.Contains(candidate.Body, "no lesson text was extracted") {
		t.Fatalf("fixture should start with the placeholder body, got %q", candidate.Body)
	}
	if err := store.PutCandidate(candidate); err != nil {
		t.Fatal(err)
	}

	approved, memory, err := ApproveCandidateWithContent(store, candidate.ID, "reviewed", CandidateContent{
		Title: "Build the check brain behind a flag",
		Body:  "Land the single-pass brain behind brain_enabled and flip the default after an A/B.",
	})
	if err != nil {
		t.Fatalf("ApproveCandidateWithContent: %v", err)
	}
	if approved.State != asymptoteobserve.LearningCandidateStateApproved || approved.MemoryID != memory.ID {
		t.Fatalf("approved = %#v", approved)
	}
	if memory.Title != "Build the check brain behind a flag" || !strings.HasPrefix(memory.Body, "Land the single-pass brain") {
		t.Fatalf("memory = %#v", memory)
	}
	if memory.Kind != candidate.Kind || memory.Applicability != candidate.Applicability || strings.Join(memory.Tags, ",") != strings.Join(candidate.Tags, ",") {
		t.Fatalf("fields not overridden must be kept: %#v", memory)
	}
	if approved.Title != memory.Title || approved.Body != memory.Body {
		t.Fatalf("candidate and memory text differ: %q vs %q", approved.Body, memory.Body)
	}
	stored, ok, err := store.GetMemory(memory.ID)
	if err != nil || !ok || stored.Body != memory.Body {
		t.Fatalf("stored memory = %#v ok=%v err=%v", stored, ok, err)
	}
	if _, _, err := ApproveCandidateWithContent(store, candidate.ID, "", CandidateContent{}); err == nil {
		t.Fatal("approving twice should fail")
	}
}

func TestApproveCandidateWithContentRejectsUnknownKind(t *testing.T) {
	store := Open(filepath.Join(t.TempDir(), "memory.db"))
	candidate, err := CandidateFromTrace(asymptoteobserve.LearningProjectV1{ID: "p"}, authoredTestTrace(), CandidateContent{Kind: "workflow", Title: "t", Body: "b"})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.PutCandidate(candidate); err != nil {
		t.Fatal(err)
	}
	if _, _, err := ApproveCandidateWithContent(store, candidate.ID, "", CandidateContent{Kind: "anecdote"}); err == nil {
		t.Fatal("expected unknown kind error")
	}
	stored, ok, err := store.GetCandidate(candidate.ID)
	if err != nil || !ok || stored.State != asymptoteobserve.LearningCandidateStateCandidate {
		t.Fatalf("candidate must stay pending after a rejected approval: %#v ok=%v err=%v", stored, ok, err)
	}
}

func TestProjectForTracePrefersExplicitPathThenRepository(t *testing.T) {
	repo := t.TempDir()
	trace := asymptoteobserve.TraceSummaryV1{Repository: &asymptoteobserve.TraceRepositoryV1{Path: repo}}
	fromTrace, err := ProjectForTrace("", trace)
	if err != nil {
		t.Fatal(err)
	}
	if fromTrace.Path != filepath.Clean(repo) {
		t.Fatalf("expected trace repository %q, got %#v", repo, fromTrace)
	}
	explicit := t.TempDir()
	fromFlag, err := ProjectForTrace(explicit, trace)
	if err != nil {
		t.Fatal(err)
	}
	if fromFlag.Path != filepath.Clean(explicit) {
		t.Fatalf("expected explicit path %q, got %#v", explicit, fromFlag)
	}
	cwd, err := ResolveProject("")
	if err != nil {
		t.Fatal(err)
	}
	fromCwd, err := ProjectForTrace("", asymptoteobserve.TraceSummaryV1{})
	if err != nil {
		t.Fatal(err)
	}
	if fromCwd.ID != cwd.ID {
		t.Fatalf("expected working-directory fallback %#v, got %#v", cwd, fromCwd)
	}
}
