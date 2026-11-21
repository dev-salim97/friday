package httpapi

import (
	"agentflow/internal/domain"
	"bytes"
	"encoding/json"
	"github.com/gin-gonic/gin"
	"testing"
)

func TestEditSharedMessageReplacesSameConversationAndRejectsStale(t *testing.T) {
	h, a, b, out, s := sharedWorkbenchFixture(t)
	s.Messages = append(s.Messages, domain.AssistantMessage{Role: "assistant", Content: "旧回复"}, domain.AssistantMessage{Role: "user", Content: "原问题"}, domain.AssistantMessage{Role: "assistant", Content: "随后回复"})
	if err := h.repo.PutAssistantSession(a.ID, s); err != nil {
		t.Fatal(err)
	}
	s, _ = h.repo.AssistantSession(a.ID, s.ID)
	body, _ := json.Marshal(gin.H{"messageIndex": 2, "content": "更新问题", "updatedAt": s.UpdatedAt})
	for _, u := range []domain.User{out, b} {
		c, w := workbenchContext(u, "POST", "/", bytes.NewBuffer(body))
		c.Params = gin.Params{{Key: "id", Value: s.ID}}
		h.editSession(c)
		want := 200
		if u.ID == out.ID {
			want = 403
		}
		if w.Code != want {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	saved, _ := h.repo.AssistantSession(a.ID, s.ID)
	if saved.ID != s.ID || saved.UserID != a.ID || len(saved.Messages) != 3 || saved.Messages[2].Content != "更新问题" || saved.Messages[2].AuthorID != b.ID || saved.ParentSessionID != "" {
		t.Fatal("edit branched or lost attribution", saved)
	}
	c, w := workbenchContext(b, "POST", "/", bytes.NewBuffer(body))
	c.Params = gin.Params{{Key: "id", Value: s.ID}}
	h.editSession(c)
	if w.Code != 409 {
		t.Fatal("stale overwrite accepted", w.Code)
	}
}
