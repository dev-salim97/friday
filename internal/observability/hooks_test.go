package observability

import (
	"strings"
	"testing"

	"agentflow/internal/config"
	"agentflow/internal/domain"
)

func TestHookEventParsesToolLifecycle(t *testing.T) {
	event, ok := HookEvent([]byte(`{"hook_event_name":"PreToolUse","tool_name":"Bash","tool_input":{"command":"git status"},"session_id":"s1","_agentflow_provider":"codex","_agentflow_hook_id":"audit","_agentflow_hook_name":"Audit"}`))
	if !ok {
		t.Fatal("expected hook event")
	}
	if event.Source.Adapter != AdapterHooks || event.Category != "tool" || event.Name != "tool.call" || event.Phase != "start" {
		t.Fatalf("unexpected event: %#v", event)
	}
	if !strings.Contains(event.Message, "Bash") || !strings.Contains(event.Message, "git status") {
		t.Fatalf("message = %q", event.Message)
	}
	if event.Source.Provider != "codex" || event.Attributes["hookId"] != "audit" || event.Attributes["hookName"] != "Audit" {
		t.Fatalf("user hook metadata missing: %#v", event)
	}
}

func TestHookEventParsesExecutionResult(t *testing.T) {
	event, ok := HookEvent([]byte(`{"hook_event_name":"Stop","_agentflow_provider":"claude-code","_agentflow_hook_id":"audit-stop","_agentflow_hook_name":"记录会话结束","_agentflow_matcher":"","_agentflow_language":"python","_agentflow_execution_phase":"finish","_agentflow_duration_ms":24,"_agentflow_exit_code":3,"_agentflow_stdout":"partial output","_agentflow_stderr":"hook failed"}`))
	if !ok {
		t.Fatal("expected hook execution event")
	}
	if event.Category != "hook" || event.Name != "hook.finish" || event.Phase != "finish" || event.Level != "error" {
		t.Fatalf("unexpected execution event: %#v", event)
	}
	if !strings.Contains(event.Message, "失败（exit 3）") || event.Attributes["durationMs"] != int64(24) || event.Attributes["stdout"] != "partial output" || event.Attributes["stderr"] != "hook failed" {
		t.Fatalf("execution result metadata missing: %#v", event)
	}
}

func TestMatchAdapter(t *testing.T) {
	live := domain.TelemetryEvent{Source: domain.TelemetrySource{Adapter: AdapterLive}}
	hooks := domain.TelemetryEvent{Source: domain.TelemetrySource{Adapter: AdapterHooks}}
	otel := domain.TelemetryEvent{Source: domain.TelemetrySource{Adapter: AdapterOTEL}}
	platform := domain.TelemetryEvent{Source: domain.TelemetrySource{Adapter: "agentflow-control-plane"}}
	cases := []struct {
		view  string
		event domain.TelemetryEvent
		want  bool
	}{
		{"live", live, true},
		{"live", platform, true},
		{"live", hooks, false},
		{"live", otel, false},
		{"hooks", hooks, true},
		{"hooks", live, false},
		{"otel", otel, true},
		{"all", hooks, true},
	}
	for _, tc := range cases {
		if got := MatchAdapter(tc.event, tc.view); got != tc.want {
			t.Fatalf("MatchAdapter(%q, %s) = %v", tc.view, tc.event.Source.Adapter, got)
		}
	}
}

func TestInjectTelemetryEnv(t *testing.T) {
	prev := config.Current()
	config.SetCurrent(config.Endpoints{CollectorBase: "http://127.0.0.1:50333"})
	t.Cleanup(func() { config.SetCurrent(prev) })
	env := map[string]string{}
	InjectTelemetryEnv(env, "run-1", "fix")
	if env["OTEL_EXPORTER_OTLP_TRACES_ENDPOINT"] != "http://127.0.0.1:50333/api/otel/v1/traces" {
		t.Fatalf("traces endpoint = %q", env["OTEL_EXPORTER_OTLP_TRACES_ENDPOINT"])
	}
	if !strings.Contains(env["OTEL_RESOURCE_ATTRIBUTES"], "agentflow.run_id=run-1") {
		t.Fatalf("resource attrs = %q", env["OTEL_RESOURCE_ATTRIBUTES"])
	}
}
