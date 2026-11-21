package httpapi

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestAssistantProgressSubmissionCannotEndToolExecution(t *testing.T) {
	h, user := workbenchFixture(t)
	gateway := &agentToolGateway{deps: h.deps}
	for _, mode := range []assistantExecMode{{}, {AutoWrite: true, ConfirmDangerous: true}, {AutoWrite: true}} {
		cancelled := false
		session := &AgentToolSession{Mode: AgentToolAssistant, ProjectID: "workbench", User: user, UserID: user.ID, ExecMode: mode, Final: make(chan AgentTerminalResult, 1), CancelModel: func() { cancelled = true }}
		result, err := gateway.callTerminal(session, submitAssistantReplyTool, json.RawMessage(`{"message":"稍等，我先看看项目里有哪些可用 SOP。","drafts":[]}`))
		if err != nil || result["isError"] != true || session.finished() || cancelled {
			t.Fatalf("progress reply terminated execution: result=%v err=%v cancelled=%v", result, err, cancelled)
		}
		if _, ok := session.takeFinal(); ok {
			t.Fatal("progress reply became the final answer")
		}
		result, err = gateway.callTerminal(session, submitAssistantReplyTool, json.RawMessage(`{"message":"创建需求","drafts":[{"body":{"title":"演练需求"}}]}`))
		encoded, _ := json.Marshal(result)
		if err != nil || result["isError"] != true || !strings.Contains(string(encoded), "tool 字段") || session.finished() || cancelled {
			t.Fatalf("missing tool did not give recoverable validation: result=%v err=%v", result, err)
		}
		// Read tools must remain callable so the model can finish preparing its draft.
		if _, err := gateway.callTool(context.Background(), session, json.RawMessage(`{"name":"workbench_tool_schema","arguments":{"name":"issues_dispatch"}}`)); err != nil {
			t.Fatalf("cannot continue querying after rejected progress: %v", err)
		}
		result, err = gateway.callTerminal(session, submitAssistantReplyTool, json.RawMessage(`{"message":"已核对需求，确认后派发。","drafts":[{"tool":"issues_dispatch","path":{"id":"workbench-issue"},"proposal":{"reason":"整件需求走 SOP","timing":"start"}}]}`))
		if err != nil || result["isError"] == true || !session.finished() || !cancelled {
			t.Fatalf("valid draft did not finish: result=%v err=%v", result, err)
		}
		final, ok := session.takeFinal()
		if !ok || final.Assistant == nil || len(final.Assistant.Drafts) != 1 {
			t.Fatal("lost valid confirmation draft")
		}
	}
}

func TestAssistantPromptsNameCallableMCPSubmissionTool(t *testing.T) {
	name := "mcp__agentflow_session__" + submitAssistantReplyTool
	for _, mode := range []string{AssistantApprovalConfirm, AssistantApprovalAssisted, AssistantApprovalAuto} {
		prompt := assistantSystemPromptForMode(mode)
		if !strings.Contains(prompt, name) || !strings.Contains(prompt, "不要把 submit_assistant_reply 裸名当成内置工具") {
			t.Fatalf("mode %s lacks qualified submission guidance", mode)
		}
	}
	if !strings.Contains(assistantNativeRetryPrompt(), name) {
		t.Fatal("terminal retry must name the actual MCP tool")
	}
	h := &assistantHandlers{}
	prompt := h.turnPrompt(assistantRequest{Messages: []assistantMessage{{Role: "user", Content: "save"}}}, t.TempDir(), true)
	if !strings.Contains(prompt, "待确认的平台写入仍须通过 "+name) {
		t.Fatal("resumed turns must refresh the qualified submission name")
	}
}

func TestAssistantDeferredWorkCannotDependOnSessionCron(t *testing.T) {
	for _, tool := range []string{"CronCreate", "CronList", "CronDelete", "Monitor"} {
		if !strings.Contains(nativeDisallowedTools, tool) {
			t.Fatalf("ephemeral tool %s remains enabled", tool)
		}
	}
	prompt := assistantSystemPromptForMode(AssistantApprovalAuto)
	if !strings.Contains(prompt, "平台持久化定时任务") || !strings.Contains(prompt, "未实际保存成功不得承诺") {
		t.Fatal("missing durable scheduling contract")
	}
}
