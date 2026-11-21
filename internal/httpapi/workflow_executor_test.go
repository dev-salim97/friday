package httpapi

import (
	"net/http"
	"testing"

	"agentflow/internal/domain"
	"agentflow/internal/repo"
)

func TestGenericTemplateAdoptsOnChosenCLI(t *testing.T) {
	h, s := newGenericTemplateServer(t)
	clonePath := "/api/workflows/" + repo.GenericBackendDeliveryWorkflowID + "/clone"
	requestJSON[map[string]string](t, h, http.MethodPost, clonePath,
		map[string]any{"id": "alpha-bad-cli", "name": "后端交付", "projectId": "alpha", "executorId": "nope"},
		http.StatusBadRequest)
	if workflowRowExists(t, s, "alpha-bad-cli") {
		t.Fatal("a rejected CLI must not create the copy")
	}
	copied := requestJSON[domain.WorkflowDefinition](t, h, http.MethodPost, clonePath,
		map[string]any{"id": "alpha-codex", "name": "后端交付", "projectId": "alpha", "executorId": "codex"},
		http.StatusCreated)
	if copied.ExecutorID != "codex" {
		t.Fatalf("adopted SOP CLI = %q, want codex", copied.ExecutorID)
	}
	// The SOP CLI is a workflow setting: the cloned roles keep their own CLI.
	if agent, ok := s.Agent(repo.GenericAgentMR); !ok || agent.ExecutorID != "claude" {
		t.Fatalf("cloned role CLI changed: %+v", agent)
	}
	if template, _ := s.GenericTemplateByWorkflow(repo.GenericBackendDeliveryWorkflowID); template.Workflow.ExecutorID != "" {
		t.Fatalf("adopting must not change the template: %q", template.Workflow.ExecutorID)
	}
	plain := requestJSON[domain.WorkflowDefinition](t, h, http.MethodPost, clonePath,
		map[string]any{"id": "alpha-plain", "name": "后端交付", "projectId": "alpha"},
		http.StatusCreated)
	if plain.ExecutorID != "" {
		t.Fatalf("omitted executorId must keep the template setting: %q", plain.ExecutorID)
	}
	recopied := requestJSON[domain.WorkflowDefinition](t, h, http.MethodPost, "/api/workflows/alpha-codex/clone",
		map[string]any{"id": "alpha-roles", "name": "按角色", "projectId": "alpha", "executorId": ""},
		http.StatusCreated)
	if recopied.ExecutorID != "" {
		t.Fatalf("empty executorId must clear the SOP CLI: %q", recopied.ExecutorID)
	}
}

func TestWorkflowSaveValidatesSOPCLI(t *testing.T) {
	h, _ := newGenericTemplateServer(t)
	copied := requestJSON[domain.WorkflowDefinition](t, h, http.MethodPost,
		"/api/workflows/"+repo.GenericBackendDeliveryWorkflowID+"/clone",
		map[string]any{"id": "alpha-save-cli", "name": "后端交付", "projectId": "alpha"},
		http.StatusCreated)
	copied.ExecutorID = "nope"
	requestJSON[map[string]string](t, h, http.MethodPut, "/api/workflows", copied, http.StatusBadRequest)
	copied.ExecutorID = " claude "
	saved := requestJSON[domain.WorkflowDefinition](t, h, http.MethodPut, "/api/workflows", copied, http.StatusOK)
	if saved.ExecutorID != "claude" {
		t.Fatalf("saved SOP CLI = %q, want claude", saved.ExecutorID)
	}
}

func TestRunStartRejectsUnknownCLI(t *testing.T) {
	h, s := newGenericTemplateServer(t)
	requestJSON[map[string]string](t, h, http.MethodPost, "/api/runs",
		map[string]any{"workflowId": repo.GenericBackendDeliveryWorkflowID, "input": "x", "projectId": "alpha", "executorId": "nope"},
		http.StatusBadRequest)
	if summaries, err := s.RunSummariesFor(domain.AnalyticsFilter{}, ""); err != nil || len(summaries) != 0 {
		t.Fatalf("a rejected CLI must not start a run: %d runs err=%v", len(summaries), err)
	}
}
