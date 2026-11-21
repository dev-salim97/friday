package httpapi

import (
	"context"
	"os"
	"strings"

	"agentflow/internal/domain"
	"agentflow/internal/runtime"

	"github.com/gin-gonic/gin"
)

const assistantDefaultModel = "default"

type assistantModel struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Family  string   `json:"family"`
	Credits string   `json:"credits"`
	Efforts []string `json:"efforts"`
	// Engine is set for models that run on a CLI other than the default one.
	Engine string `json:"engine,omitempty"`
}

type assistantModelCatalog struct {
	Default        string           `json:"default"`
	DefaultEfforts []string         `json:"defaultEfforts,omitempty"`
	Models         []assistantModel `json:"models"`
	// Engines lists the CLIs a signed-in user can switch between.
	Engines             []assistantEngineOption `json:"engines,omitempty"`
	CursorModelsPending bool                    `json:"cursorModelsPending,omitempty"`
}

// assistantKnownCatalog validates settings that are not tied to one user,
// such as SOP roles and notification bots.
func assistantKnownCatalog() assistantModelCatalog {
	return assistantFixedCatalog()
}

// models returns the chat model catalog. Guests get the configured models
// only; actually chatting still requires login plus a configured key.
func (h *assistantHandlers) models(c *gin.Context) {
	if raw := strings.TrimSpace(c.Query("executor")); raw != "" {
		engine := assistantExecutor(raw)
		id := engine
		if engine == assistantEngineClaude {
			id = "claude"
		} else if id == "claude" {
			engine = assistantEngineClaude
		}
		var user domain.User
		if h.repo != nil && (engine == runtime.AgentCLIKimi || engine == runtime.AgentCLIAntigravity) {
			if current, signedIn := h.requestUser(c.Request, true); signedIn {
				user, _ = h.repo.User(current.ID)
			}
		}
		_, kimiBaseURL := assistantEngineCredential(user, runtime.AgentCLIKimi)
		catalog, ok := executorModelCatalogs(kimiBaseURL)[id]
		if !ok {
			c.JSON(400, gin.H{"error": "不支持的执行器"})
			return
		}
		result := assistantModelCatalog{Models: make([]assistantModel, 0, len(catalog.Models))}
		if engine == runtime.AgentCLIAntigravity && antigravityUsesCustomEndpoint(user) {
			c.Header("Cache-Control", "private, no-store")
			c.JSON(200, result)
			return
		}
		for _, item := range catalog.Models {
			levels := make([]string, 0, len(item.Efforts))
			for _, option := range item.Efforts {
				levels = append(levels, option.Value)
			}
			result.Models = append(result.Models, assistantModel{ID: item.ID, Name: item.Name, Family: item.Family, Engine: engine, Efforts: levels})
		}
		if len(result.Models) > 0 {
			result.Default = result.Models[0].ID
		}
		c.JSON(200, result)
		return
	}

	catalog := assistantChatCatalog(c.Request.Context())
	user, signedIn := domain.User{}, false
	if current, ok := h.requestUser(c.Request, true); ok {
		user, signedIn = h.repo.User(current.ID)
	}
	catalog.Models = selectableDefaultModels(catalog.Models)
	if signedIn {
		c.Header("Cache-Control", "private, no-store")
		catalog.Engines = assistantEngineOptions(user)
		var runnerLogin map[string]bool
		if c.Query("schedule") == "1" {
			runnerLogin = make(map[string]bool)
			for i := range catalog.Engines {
				engine := catalog.Engines[i].ID
				if scheduleLoginRunner(h.repo, user.ID, engine).ID != "" {
					runnerLogin[engine] = true
					catalog.Engines[i].Configured = true
				}
			}
		}
		models, pending := assistantEngineModels(c.Request.Context(), user, runnerLogin)
		catalog.Models = append(catalog.Models, models...)
		catalog.CursorModelsPending = pending
		h.saveCursorCatalog(user)
	}
	c.JSON(200, catalog)
}

// Keep the CLI default as an execution fallback and for older saved records.
// The model pickers offer the concrete models available to the caller.
func selectableDefaultModels(models []assistantModel) []assistantModel {
	listed := make([]assistantModel, 0, len(models))
	for _, model := range models {
		if !assistantIsDefaultModel(model.ID) {
			listed = append(listed, model)
		}
	}
	return listed
}

func assistantIsDefaultModel(id string) bool {
	return id == assistantDefaultModel || id == "default-model"
}

// assistantChatCatalog is the catalog used by chat conversations.
func assistantChatCatalog(_ context.Context) assistantModelCatalog {
	return assistantFixedCatalog()
}

// assistantFixedCatalog is the operator-configured catalog for unprefixed
// model ids (AGENTFLOW_ASSISTANT_MODEL / AGENTFLOW_ASSISTANT_MODELS).
func assistantFixedCatalog() assistantModelCatalog {
	defaultID := strings.TrimSpace(os.Getenv("AGENTFLOW_ASSISTANT_MODEL"))
	if defaultID == "" {
		defaultID = assistantDefaultModel
	}
	defaultName := defaultID
	if defaultID == assistantDefaultModel {
		defaultName = "CLI 默认模型"
	}
	models := []assistantModel{assistantListedModel(defaultID, defaultName, "")}
	seen := map[string]bool{defaultID: true}
	for _, id := range strings.Split(os.Getenv("AGENTFLOW_ASSISTANT_MODELS"), ",") {
		id = strings.TrimSpace(id)
		if id != "" && !seen[id] {
			models = append(models, assistantListedModel(id, id, ""))
			seen[id] = true
		}
	}
	return assistantModelCatalog{Default: defaultID, Models: models}
}

func assistantListedModel(id, name, credits string) assistantModel {
	return assistantModel{ID: id, Name: name, Family: assistantModelFamily(id, assistantDefaultEngine), Credits: credits, Efforts: assistantModelEffortsIn(id, nil)}
}

func assistantModelFamily(id, engine string) string {
	id = strings.ToLower(strings.TrimSpace(id))
	id = id[strings.LastIndexByte(id, '/')+1:]
	switch {
	case strings.HasPrefix(id, "claude"):
		return "claude"
	case strings.HasPrefix(id, "gpt") || strings.Contains(id, "codex"):
		return "gpt"
	case strings.HasPrefix(id, "glm"):
		return "glm"
	case strings.HasPrefix(id, "gemini"):
		return "gemini"
	case id == "grok" || strings.HasPrefix(id, "grok-") || strings.HasPrefix(id, "cursor-grok-"):
		return "grok"
	case id == "muse" || strings.HasPrefix(id, "muse-"):
		return "muse"
	case id == "composer" || strings.HasPrefix(id, "composer-") || (engine == runtime.AgentCLICursor && (id == "auto" || id == "auto-smart")):
		return "cursor"
	case strings.HasPrefix(id, "kimi") || id == "k3" || strings.HasPrefix(id, "k3-") || strings.HasPrefix(id, "k3."):
		return "kimi"
	case strings.HasPrefix(id, "moonshot"):
		return "moonshot"
	case strings.HasPrefix(id, "qwen") || strings.HasPrefix(id, "qvq"):
		return "qwen"
	case strings.HasPrefix(id, "doubao") || strings.HasPrefix(id, "seed-") || strings.HasPrefix(id, "seedream"):
		return "doubao"
	case strings.HasPrefix(id, "ernie") || strings.HasPrefix(id, "wenxin") || strings.HasPrefix(id, "qianfan"):
		return "qianfan"
	case strings.HasPrefix(id, "deepseek"):
		return "deepseek"
	case strings.HasPrefix(id, "minimax"):
		return "minimax"
	case strings.HasPrefix(id, "hy") || strings.Contains(id, "hunyuan"):
		return "hunyuan"
	case strings.HasPrefix(id, "mimo"):
		return "mimo"
	default:
		return "other"
	}
}

func assistantAllowedModel(id string, catalog assistantModelCatalog) string {
	id = strings.TrimSpace(id)
	if id == "" {
		return catalog.Default
	}
	for _, model := range catalog.Models {
		if model.ID == id {
			return id
		}
	}
	return catalog.Default
}
