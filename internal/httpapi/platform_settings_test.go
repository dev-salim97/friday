package httpapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"agentflow/internal/domain"
	"agentflow/internal/engine"
	"agentflow/internal/repo"
	"agentflow/internal/runtime"
)

type platformSettingsClient struct {
	t *testing.T
	h http.Handler
}

func (p platformSettingsClient) call(cookie *http.Cookie, method, path, origin, body string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, "http://localhost"+path, bytes.NewBufferString(body))
	r.Header.Set("Content-Type", "application/json")
	if origin != "" {
		r.Header.Set("Origin", origin)
	}
	if cookie != nil {
		r.AddCookie(cookie)
	}
	w := httptest.NewRecorder()
	p.h.ServeHTTP(w, r)
	return w
}

// platformSettingsSignIns gives every sign-in its own client address, so these
// tests never use up the per-IP password budget other tests share.
var platformSettingsSignIns atomic.Int32

func (p platformSettingsClient) signIn(login, password string) *http.Cookie {
	p.t.Helper()
	r := httptest.NewRequest("POST", "http://localhost/api/auth/password", bytes.NewBufferString(`{"login":"`+login+`","password":"`+password+`"}`))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Origin", "http://localhost")
	r.RemoteAddr = fmt.Sprintf("198.51.100.%d:1234", platformSettingsSignIns.Add(1)%250+1)
	w := httptest.NewRecorder()
	p.h.ServeHTTP(w, r)
	if w.Code != 200 || len(w.Result().Cookies()) == 0 {
		p.t.Fatalf("sign in %s: %d %s", login, w.Code, w.Body.String())
	}
	return w.Result().Cookies()[0]
}

// With self-service signup closed, administrators create accounts and reset
// passwords from 平台管理 instead of the command line.
func TestPlatformAdminsManageAccounts(t *testing.T) {
	s, client, owner, member := platformSettingsFixture(t)
	if err := s.PutProject(domain.Project{ID: welcomeProjectID, Name: "Welcome"}); err != nil {
		t.Fatal(err)
	}
	const origin = "http://localhost"
	create := `{"loginName":"alice","password":"alice-initial","admin":false}`

	if w := client.call(member, "GET", "/api/platform/accounts", "", ""); w.Code != 403 {
		t.Fatalf("member list: %d", w.Code)
	}
	if w := client.call(member, "POST", "/api/platform/accounts", origin, create); w.Code != 403 {
		t.Fatalf("member create: %d", w.Code)
	}
	if w := client.call(owner, "POST", "/api/platform/accounts", "https://attacker.invalid", create); w.Code != 403 {
		t.Fatalf("cross-origin create: %d", w.Code)
	}
	if w := client.call(owner, "POST", "/api/platform/accounts", "", create); w.Code != 403 {
		t.Fatalf("create without origin: %d", w.Code)
	}
	for _, bad := range []string{`{"loginName":"a","password":"alice-initial"}`, `{"loginName":"bad name","password":"alice-initial"}`, `{"loginName":"alice","password":"short"}`} {
		if w := client.call(owner, "POST", "/api/platform/accounts", origin, bad); w.Code != 400 {
			t.Fatalf("invalid %s: %d %s", bad, w.Code, w.Body.String())
		}
	}
	w := client.call(owner, "POST", "/api/platform/accounts", origin, create)
	var created repo.PlatformAccount
	if w.Code != 201 || json.Unmarshal(w.Body.Bytes(), &created) != nil || created.LoginName != "alice" || !created.Password || created.Admin {
		t.Fatalf("create: %d %s", w.Code, w.Body.String())
	}
	if strings.Contains(w.Body.String(), "alice-initial") {
		t.Fatal("responses must not echo the password")
	}
	if w := client.call(owner, "POST", "/api/platform/accounts", origin, create); w.Code != 409 {
		t.Fatalf("duplicate: %d", w.Code)
	}
	if !s.CheckLocalPassword(created.UserID, "alice-initial") {
		t.Fatal("the new account signs in with the initial password")
	}
	welcome, _ := s.Project(welcomeProjectID)
	if len(welcome.Members) != 1 || welcome.Members[0].UserID != created.UserID {
		t.Fatalf("new account joins the welcome project: %+v", welcome.Members)
	}

	w = client.call(owner, "GET", "/api/platform/accounts", "", "")
	var listed struct {
		Accounts      []repo.PlatformAccount
		SignupEnabled bool
	}
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &listed) != nil || listed.SignupEnabled {
		t.Fatalf("list: %d %s", w.Code, w.Body.String())
	}
	byLogin := map[string]repo.PlatformAccount{}
	for _, account := range listed.Accounts {
		byLogin[account.LoginName] = account
	}
	if !byLogin["owner"].Admin || byLogin["member"].Admin || !byLogin["alice"].Password || listed.Accounts[len(listed.Accounts)-1].LoginName != "alice" {
		t.Fatalf("accounts = %+v", listed.Accounts)
	}

	reset := `{"password":"alice-second"}`
	path := "/api/platform/accounts/" + itoa(created.UserID) + "/password"
	if w := client.call(member, "PUT", path, origin, reset); w.Code != 403 {
		t.Fatalf("member reset: %d", w.Code)
	}
	if w := client.call(owner, "PUT", path, origin, `{"password":"short"}`); w.Code != 400 {
		t.Fatalf("short reset: %d", w.Code)
	}
	if w := client.call(owner, "PUT", path, origin, reset); w.Code != 200 {
		t.Fatalf("reset: %d %s", w.Code, w.Body.String())
	}
	if s.CheckLocalPassword(created.UserID, "alice-initial") || !s.CheckLocalPassword(created.UserID, "alice-second") {
		t.Fatal("reset replaces the password")
	}
	if w := client.call(owner, "PUT", "/api/platform/accounts/999999/password", origin, reset); w.Code != 404 {
		t.Fatalf("missing account: %d", w.Code)
	}
	if err := s.PutUser(domain.User{LoginName: "octocat", ChineseName: "octocat"}); err != nil {
		t.Fatal(err)
	}
	octocat, _ := s.UserByLoginName("octocat")
	if w := client.call(owner, "PUT", "/api/platform/accounts/"+itoa(octocat.ID)+"/password", origin, reset); w.Code != 400 {
		t.Fatalf("a GitHub account must not gain a password login: %d", w.Code)
	}
}

func itoa(v int64) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func platformSettingsFixture(t *testing.T) (*repo.Store, platformSettingsClient, *http.Cookie, *http.Cookie) {
	t.Helper()
	t.Setenv("AGENTFLOW_AUTH_REQUIRED", "true")
	t.Setenv("AGENTFLOW_SCHEDULER_ENABLED", "false")
	t.Setenv("AGENTFLOW_ALLOW_SIGNUP", "false")
	s, err := repo.OpenTest()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	if _, err := s.CreateLocalAccount("owner", "owner-password", true); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateLocalAccount("member", "member-password", false); err != nil {
		t.Fatal(err)
	}
	client := platformSettingsClient{t: t, h: New(s, engine.New(s, runtime.NewManager(t.TempDir())), nil, "", "")}
	return s, client, client.signIn("owner", "owner-password"), client.signIn("member", "member-password")
}
