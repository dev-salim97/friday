package httpapi

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"agentflow/internal/domain"
	"agentflow/internal/repo"
	"agentflow/internal/tracker"
)

const liveTAPDStoryURL = "https://tapd.example.com/tapd_fe/10000000/story/detail/1010000000100000555"

func TestMySQLImportTAPDAndSanitizeClarifyOptions(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("AGENTFLOW_TEST_MYSQL_DSN"))
	token := strings.TrimSpace(os.Getenv("AGENTFLOW_MCP_TOKEN"))
	if dsn == "" || token == "" {
		t.Skip("AGENTFLOW_TEST_MYSQL_DSN and AGENTFLOW_MCP_TOKEN required")
	}
	st, err := repo.Open(dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })

	login := fmt.Sprintf("clarify-opt-%d", time.Now().UnixNano())
	if err := st.PutUser(domain.User{LoginName: login, ChineseName: "clarify-opt"}); err != nil {
		t.Fatal(err)
	}
	user, ok := st.UserByLoginName(login)
	if !ok {
		t.Fatal("user missing")
	}
	user, _ = st.UserByLoginName(login)
	projectID := "clarify-opt"
	if err := st.PutProject(domain.Project{
		ID: projectID, Key: "COP", Name: "clarify options", OwnerID: user.ID,
		Members: []domain.ProjectMember{{UserID: user.ID, Role: "maintainer"}},
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = st.DeleteIssue("tapd-1010000000100000555")
		_ = st.DB.Exec("DELETE FROM mcp_servers WHERE project_id=?", projectID)
		_ = st.DB.Exec("DELETE FROM projects WHERE id=?", projectID)
		_ = st.DB.Exec("DELETE FROM users WHERE id=?", user.ID)
	})
	if err := st.PutMCPServer(domain.MCPServerDefinition{
		ID: "mcp-70-project-" + projectID, Name: "TAPD MCP", MarketID: "70",
		Scope: "project", ProjectID: projectID, Transport: "http",
		URL: "https://mcp-gateway.example.com/tapd/", Headers: map[string]string{"Authorization": "Bearer " + token}, TimeoutSec: 60,
	}); err != nil {
		t.Fatal(err)
	}
	svc := tracker.NewService()
	svc.ConfigureWithStore(st.SyncConfig(), st)
	issue, err := svc.ImportTAPDStory(st, projectID, liveTAPDStoryURL, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(issue.Title, "国际站") {
		t.Fatalf("title=%q", issue.Title)
	}
	if strings.TrimSpace(issue.Description) == "" {
		t.Fatal("imported TAPD description is empty")
	}

	h := &streamHandlers{deps: &deps{repo: st, sync: svc}}
	dump := `{"type":"conversation_record","id":"tapd-chat","payload":{"comments":[{"body":"DescribeDomainLastPackageInfo"}]}}`
	parsed := map[string]any{
		"summary": "国际站取回绑定套餐应对齐国内站",
		"questions": []any{
			map[string]any{
				"id":       "q1",
				"question": "取回时套餐如何处理？",
				"options": []any{
					"付费套餐取回后自动解绑为免费版",
					map[string]any{"label": "保持现有付费套餐不能取回"},
					dump,
				},
			},
		},
		"ready": false,
	}
	stored, err := h.persistClarifyOutcome(issue.ID, parsed, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(stored.Clarification.Questions) != 1 {
		t.Fatalf("questions=%+v", stored.Clarification.Questions)
	}
	wantOpts := []string{"付费套餐取回后自动解绑为免费版", "保持现有付费套餐不能取回"}
	if got := stored.Clarification.Questions[0].Options; len(got) != 2 || got[0] != wantOpts[0] || got[1] != wantOpts[1] {
		t.Fatalf("options=%v", got)
	}
	for _, opt := range stored.Clarification.Questions[0].Options {
		if strings.Contains(opt, "conversation_record") || strings.Contains(opt, "{") {
			t.Fatalf("json dump leaked into option %q", opt)
		}
	}

	reopened, err := repo.Open(dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { reopened.Close() })
	got, ok := reopened.Issue(issue.ID)
	if !ok {
		t.Fatal("reloaded issue missing")
	}
	if len(got.Clarification.Questions) != 1 {
		t.Fatalf("reloaded questions=%+v", got.Clarification.Questions)
	}
	if opts := got.Clarification.Questions[0].Options; len(opts) != 2 || opts[0] != wantOpts[0] || opts[1] != wantOpts[1] {
		t.Fatalf("reloaded options=%v", opts)
	}
	if !strings.Contains(got.Description, "国际站") && !strings.Contains(got.Description, "套餐") {
		t.Fatalf("reloaded description missing TAPD body: %q", got.Description)
	}
}

func TestSeedLocalClarifyPreview(t *testing.T) {
	if os.Getenv("AGENTFLOW_SEED_LOCAL") != "1" {
		t.Skip("set AGENTFLOW_SEED_LOCAL=1 to seed the local preview database")
	}
	loadAcceptanceDotEnv()
	dsn := strings.TrimSpace(os.Getenv("AGENTFLOW_TEST_MYSQL_DSN"))
	if dsn == "" {
		dsn = strings.TrimSpace(os.Getenv("AGENTFLOW_MYSQL_DSN"))
	}
	token := strings.TrimSpace(os.Getenv("AGENTFLOW_MCP_TOKEN"))
	if dsn == "" || token == "" {
		t.Fatal("AGENTFLOW_MYSQL_DSN and AGENTFLOW_MCP_TOKEN required")
	}
	st, err := repo.Open(dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	if err := st.PutUser(domain.User{LoginName: "example-maintainer", ChineseName: "示例用户"}); err != nil {
		t.Fatal(err)
	}
	user, ok := st.UserByLoginName("example-maintainer")
	if !ok {
		t.Fatal("user missing")
	}
	user, _ = st.UserByLoginName("example-maintainer")
	if err := st.PutProject(domain.Project{
		ID: "dp", Key: "DP", Name: "DemoDNS", Color: "#536dfe", OwnerID: user.ID,
		Members: []domain.ProjectMember{{UserID: user.ID, Role: "maintainer"}},
	}); err != nil {
		t.Fatal(err)
	}
	if err := st.PutMCPServer(domain.MCPServerDefinition{
		ID: "mcp-70-project-dp", Name: "TAPD MCP", MarketID: "70",
		Scope: "project", ProjectID: "dp", Transport: "http",
		URL: "https://mcp-gateway.example.com/tapd/", Headers: map[string]string{"Authorization": "Bearer " + token}, TimeoutSec: 60,
	}); err != nil {
		t.Fatal(err)
	}
	svc := tracker.NewService()
	svc.ConfigureWithStore(st.SyncConfig(), st)
	issue, err := svc.ImportTAPDStory(st, "dp", liveTAPDStoryURL, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	h := &streamHandlers{deps: &deps{repo: st, sync: svc}}
	dump := `{"type":"conversation_record","id":"tapd-chat","payload":{"comments":[{"body":"DescribeDomainLastPackageInfo"}]}}`
	if _, err := h.persistClarifyOutcome(issue.ID, map[string]any{
		"summary": "国际站取回绑定套餐应对齐国内站",
		"questions": []any{map[string]any{
			"id": "q1", "question": "取回时套餐如何处理？",
			"options": []any{"付费套餐取回后自动解绑为免费版", map[string]any{"label": "保持现有付费套餐不能取回"}, dump},
		}},
		"ready": false,
	}, ""); err != nil {
		t.Fatal(err)
	}
	stored, _ := st.Issue(issue.ID)
	stored.Clarification.Questions[0].Options = append(stored.Clarification.Questions[0].Options, dump)
	if err := st.PutIssue(stored); err != nil {
		t.Fatal(err)
	}
	t.Logf("seeded %s (%s) with %d options (last is a JSON dump for frontend filter)", stored.ID, stored.Title, len(stored.Clarification.Questions[0].Options))
}

func loadAcceptanceDotEnv() {
	if os.Getenv("AGENTFLOW_LIVE_TESTS") != "1" {
		return
	}
	var candidates []string
	if cwd, err := os.Getwd(); err == nil {
		dir := cwd
		for i := 0; i < 6; i++ {
			candidates = append(candidates, filepath.Join(dir, ".env"))
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}
	for _, p := range candidates {
		b, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(b), "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			line = strings.TrimPrefix(line, "export ")
			k, v, ok := strings.Cut(line, "=")
			if !ok {
				continue
			}
			k = strings.TrimSpace(k)
			v = strings.Trim(strings.TrimSpace(v), `"'`)
			if k == "" || os.Getenv(k) != "" {
				continue
			}
			switch strings.ToUpper(k) {
			case "HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY", "NO_PROXY":
				continue
			}
			os.Setenv(k, v)
		}
		return
	}
}
