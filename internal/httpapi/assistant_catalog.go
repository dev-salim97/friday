package httpapi

import (
	"context"

	"agentflow/internal/domain"
)

// chatCatalog is the chat catalog for a chat user.
func (h *assistantHandlers) chatCatalog(ctx context.Context, _ domain.User) assistantModelCatalog {
	return assistantChatCatalog(ctx)
}

// turnCatalog is chatCatalog for a turn on model.
func (h *assistantHandlers) turnCatalog(ctx context.Context, user domain.User, _ string) assistantModelCatalog {
	return h.chatCatalog(ctx, user)
}

// loadAssistantModels returns the operator-configured catalog of unprefixed
// models; per-CLI models come from the caller's own configuration.
func loadAssistantModels(ctx context.Context) assistantModelCatalog {
	return assistantChatCatalog(ctx)
}

// assistantModelCredentialConfigured reports whether the user saved a key for
// the CLI that runs model.
func assistantModelCredentialConfigured(user domain.User, model string) bool {
	engine, _ := assistantModelEngine(model)
	key, _ := assistantEngineCredential(user, engine)
	return key != ""
}
