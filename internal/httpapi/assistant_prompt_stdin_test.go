package httpapi

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agentflow/internal/runtime"
)

// Linux rejects any single argv string above MAX_ARG_STRLEN (128KiB).
const testMaxArgStrlen = 128 << 10

// A fresh chat session embeds the whole conversation in the prompt. Once it
// passes MAX_ARG_STRLEN the CLI failed with "fork/exec ...: argument list too
// long" before starting, and every retry failed the same way.
func oversizedAssistantPrompt() string {
	return strings.Repeat(`{"role":"user","content":"很长的对话历史"}`, 200<<10/48)
}

func assertPromptOnStdin(t *testing.T, name string, command runtime.CommandSpec, prompt string) {
	t.Helper()
	for _, arg := range command.Argv {
		if len(arg) >= testMaxArgStrlen {
			t.Fatalf("%s: argv still carries a %d-byte argument", name, len(arg))
		}
	}
	if command.Argv[len(command.Argv)-1] != "--" {
		t.Fatalf("%s: argv should end with the -- terminator, got %q", name, command.Argv[len(command.Argv)-1])
	}
	if command.Stdin == nil {
		t.Fatalf("%s: oversized prompt was not moved to stdin", name)
	}
	data, err := io.ReadAll(command.Stdin)
	if err != nil || string(data) != prompt {
		t.Fatalf("%s: stdin carries %d bytes (err=%v), want %d", name, len(data), err, len(prompt))
	}
}

func TestAssistantClaudeOversizedPromptUsesStdin(t *testing.T) {
	root := t.TempDir()
	config := filepath.Join(root, "mcp.json")
	if err := os.WriteFile(config, []byte(`{"mcpServers":{}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	prompt := oversizedAssistantPrompt()
	inv, err := prepareAssistantClaudeInvocation(assistantEngineInvocationOpts{Bin: "claude", Root: root, Config: config, Prompt: prompt, Model: "m", SessionID: "s"})
	if err != nil {
		t.Fatal(err)
	}
	assertPromptOnStdin(t, "claude", inv.command, prompt)
}

func TestClarifyOversizedPromptUsesStdin(t *testing.T) {
	prompt := oversizedAssistantPrompt()
	mcp := filepath.Join(t.TempDir(), "mcp.json")
	if err := os.WriteFile(mcp, []byte(`{"mcpServers":{}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	command, err := prepareClarifyCommandWith(clarifyCommandOpts{Bin: "claude", Prompt: prompt, SessionID: "s", Native: true, MCPConfig: mcp, SubmitOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	assertPromptOnStdin(t, "clarify", command, prompt)
}
