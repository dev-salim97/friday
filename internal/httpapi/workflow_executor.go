package httpapi

import (
	"fmt"
	"strings"

	"agentflow/internal/repo"
)

// validateWorkflowExecutor checks a SOP-wide 执行 CLI: a saved
// WorkflowDefinition.ExecutorID or a run-level choice. Empty keeps every role
// on its own CLI; anything else must be a built-in, user-selectable CLI.
func validateWorkflowExecutor(id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil
	}
	if _, ok := repo.BuiltInExecutor(id); !ok {
		return fmt.Errorf("不支持的 CLI：%s", id)
	}
	return nil
}
