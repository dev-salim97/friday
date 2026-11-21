package httpapi

import (
	"agentflow/internal/domain"
	"agentflow/internal/notification"
	"agentflow/internal/repo"
	"context"
	"encoding/json"
	"errors"
	"testing"
)

func activeReportSession(t *testing.T) *AgentToolSession {
	t.Helper()
	agentToolSessions.mu.Lock()
	defer agentToolSessions.mu.Unlock()
	for _, s := range agentToolSessions.sessions {
		if s.Mode == AgentToolReport {
			return s
		}
	}
	t.Fatal("missing report session")
	return nil
}

func TestSOPReportMissingSubmissionRetriesWithoutParsingText(t *testing.T) {
	store, _ := repo.OpenTest()
	defer store.Close()
	store.PutUser(domain.User{ID: 42, LoginName: "executor", ClaudeCodeAPIKey: "key"})
	h := &assistantHandlers{deps: &deps{repo: store}}
	calls := 0
	h.generate = func(ctx context.Context, prompt string, user domain.User) (string, error) {
		calls++
		return "```notification\n{\"body\":\"looks valid\"}\n```", nil
	}
	_, err := h.generateSOPReport(context.Background(), domain.Run{ExecutionContext: domain.TelemetryContext{ResponsibleUserID: 42}}, domain.DefaultNotificationReportModel, "资料")
	if !errors.Is(err, notification.ErrReportFormat) || calls != nativeTerminalMaxRetries+1 {
		t.Fatalf("calls=%d err=%v", calls, err)
	}
	calls = 0
	h.generate = func(ctx context.Context, prompt string, user domain.User) (string, error) {
		calls++
		session := activeReportSession(t)
		gateway := &agentToolGateway{deps: h.deps}
		args := json.RawMessage(`{"name":"submit_notification_report","arguments":{"body":" "}}`)
		if calls > 1 {
			args = json.RawMessage(`{"name":"submit_notification_report","arguments":{"body":"补交成功"}}`)
		}
		_, err := gateway.callTool(ctx, session, args)
		return "", err
	}
	report, err := h.generateSOPReport(context.Background(), domain.Run{ExecutionContext: domain.TelemetryContext{ResponsibleUserID: 42}}, domain.DefaultNotificationReportModel, "资料")
	if err != nil || calls != 2 || report.Body != "补交成功" {
		t.Fatalf("repair report=%+v calls=%d err=%v", report, calls, err)
	}

}

func TestSubmitNotificationReportValidationAndModeIsolation(t *testing.T) {
	gateway := &agentToolGateway{}
	session := &AgentToolSession{Mode: AgentToolReport, Final: make(chan AgentTerminalResult, 1)}
	for _, body := range []string{`{"body":" "}`, `{"body":"报告","runId":"other-run"}`, `{"body":2}`, `{"body":"报告"} {}`} {
		args := json.RawMessage(`{"name":"submit_notification_report","arguments":` + body + `}`)
		// The trailing object is directly tested by the argument decoder below.
		if json.Valid(args) {
			result, err := gateway.callTool(context.Background(), session, args)
			if err == nil && result["isError"] != true {
				t.Fatalf("accepted %s", body)
			}
		}
		if _, err := parseSubmitNotificationReportArgs(json.RawMessage(body)); err == nil {
			t.Fatalf("accepted invalid args %s", body)
		}
		if session.finished() {
			t.Fatal("invalid submission ended session")
		}
	}
	for _, mode := range []AgentToolMode{AgentToolAssistant, AgentToolClarify, AgentToolScheduled} {
		other := &AgentToolSession{Mode: mode, Final: make(chan AgentTerminalResult, 1)}
		if _, err := gateway.callTool(context.Background(), other, json.RawMessage(`{"name":"submit_notification_report","arguments":{"body":"报告"}}`)); err == nil {
			t.Fatalf("mode %s accepted report", mode)
		}
	}
	valid := json.RawMessage(`{"name":"submit_notification_report","arguments":{"body":"报告"}}`)
	if result, err := gateway.callTool(context.Background(), session, valid); err != nil || result["isError"] == true {
		t.Fatalf("valid failed %v", err)
	}
}
