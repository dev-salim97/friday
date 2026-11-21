package engine

import (
	"strings"
	"testing"
	"time"

	"agentflow/internal/domain"
	"agentflow/internal/repo"
	"agentflow/internal/runtime"
)

func TestResolveWorkflowNodeExecutorAppliesSOPCLI(t *testing.T) {
	t.Parallel()
	s, err := repo.OpenTest()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	role := domain.AgentDefinition{ID: "dev", Name: "Dev", ExecutorID: "claude", Model: "cb-model", Effort: "high"}
	node := domain.WorkflowNode{ID: "implement", Type: "agent", AgentID: role.ID}

	own, err := resolveWorkflowNodeExecutor(s, domain.WorkflowDefinition{}, node, role)
	if err != nil || own.ID != "claude" || own.Model != "cb-model" || own.Effort != "high" {
		t.Fatalf("empty SOP CLI must keep the role CLI: %+v err=%v", own, err)
	}
	other, err := resolveWorkflowNodeExecutor(s, domain.WorkflowDefinition{ExecutorID: "codex"}, node, role)
	if err != nil {
		t.Fatal(err)
	}
	if other.ID != "codex" || other.Kind != "codex" || other.Binary != "codex" {
		t.Fatalf("SOP CLI must replace the role CLI: %+v", other)
	}
	if other.Model != "" || other.Effort != "" {
		t.Fatalf("a Claude Code model must not leak into Codex: %+v", other)
	}
	same, err := resolveWorkflowNodeExecutor(s, domain.WorkflowDefinition{ExecutorID: "claude"}, node, role)
	if err != nil || same.Model != "cb-model" || same.Effort != "high" {
		t.Fatalf("same-kind SOP CLI must keep the role model and effort: %+v err=%v", same, err)
	}

	previewNode := domain.WorkflowNode{ID: domain.PreviewBuildNodeID, Type: "agent", AgentID: domain.PreviewBuildAgentID}
	preview, err := resolveWorkflowNodeExecutor(s, domain.WorkflowDefinition{ExecutorID: "codex"}, previewNode, domain.PreviewBuildAgent(nil))
	if err != nil || preview.Kind != "claude-code" {
		t.Fatalf("preview build node must stay on Claude Code: %+v err=%v", preview, err)
	}

	if _, err := resolveWorkflowNodeExecutor(s, domain.WorkflowDefinition{ExecutorID: "nope"}, node, role); err == nil || !strings.Contains(err.Error(), "SOP 执行 CLI") {
		t.Fatalf("unknown SOP CLI must be rejected: %v", err)
	}
}

func TestStartWithExecutorFreezesRunOnlyCLI(t *testing.T) {
	t.Parallel()
	s, err := repo.OpenTest()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.PutAgent(domain.AgentDefinition{ID: "cli-role", Name: "Role", ProjectID: "p", ExecutorID: "claude"}); err != nil {
		t.Fatal(err)
	}
	wf := domain.WorkflowDefinition{ID: "cli-sop", Name: "CLI", SandboxProfileID: "local", Nodes: []domain.WorkflowNode{{ID: "work", Name: "work", Type: "agent", AgentID: "cli-role", MaxAttempts: 1}}}
	if err := s.PutWorkflow(wf); err != nil {
		t.Fatal(err)
	}
	e := New(s, runtime.NewManager(t.TempDir()))
	e.ConfigureExecution("cli-worker", "cli-host", time.Minute, false)

	run, err := e.StartWithExecutor(wf.ID, "", nil, domain.TelemetryContext{ProjectID: "p"}, "codex")
	if err != nil {
		t.Fatal(err)
	}
	if run.WorkflowSnapshot.ExecutorID != "codex" {
		t.Fatalf("run snapshot CLI = %q, want codex", run.WorkflowSnapshot.ExecutorID)
	}
	if stored, _ := s.Workflow(wf.ID); stored.ExecutorID != "" {
		t.Fatalf("a run-only CLI must not change the SOP: %q", stored.ExecutorID)
	}
	if ctx := e.observationContext(run, "work", nil, domain.TelemetryEvent{}); ctx.ExecutorID != "codex" {
		t.Fatalf("telemetry executor = %q, want codex", ctx.ExecutorID)
	}

	plain, err := e.StartWithExecutor(wf.ID, "", nil, domain.TelemetryContext{ProjectID: "p"}, "")
	if err != nil || plain.WorkflowSnapshot.ExecutorID != "" {
		t.Fatalf("empty run CLI must follow the SOP: %q err=%v", plain.WorkflowSnapshot.ExecutorID, err)
	}
	if _, err := e.StartWithExecutor(wf.ID, "", nil, domain.TelemetryContext{ProjectID: "p"}, "nope"); err == nil {
		t.Fatal("unknown run CLI must be rejected")
	}
}
