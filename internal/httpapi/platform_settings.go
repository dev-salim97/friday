package httpapi

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"agentflow/internal/repo"

	"github.com/gin-gonic/gin"
)

// 平台管理 → 成员账号. Only platform
// administrators may use these endpoints.

// platformAdminWrite admits a platform administrator's change: an API token,
// or a same-origin browser request (the session cookie alone is not enough).
func (h *usersHandlers) platformAdminWrite(c *gin.Context) bool {
	if !h.isPlatformAdmin(c.Request) {
		c.JSON(http.StatusForbidden, gin.H{"error": "只有平台管理员可以执行该操作"})
		return false
	}
	if _, bearer := bearerToken(c.Request); bearer {
		return true
	}
	origin, err := url.Parse(c.GetHeader("Origin"))
	if err != nil || origin.Host != c.Request.Host || (origin.Scheme != "http" && origin.Scheme != "https") {
		c.JSON(http.StatusForbidden, gin.H{"error": "invalid request origin"})
		return false
	}
	return true
}

func (h *usersHandlers) platformAccounts(c *gin.Context) {
	if c.Request.Method != http.MethodGet {
		h.createPlatformAccount(c)
		return
	}
	if !h.isPlatformAdmin(c.Request) {
		c.JSON(http.StatusForbidden, gin.H{"error": "只有平台管理员可以查看成员账号"})
		return
	}
	accounts, err := h.repo.ListPlatformAccounts()
	if err != nil {
		respond(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"accounts": accounts, "signupEnabled": signupEnabled()})
}

func validAccountPassword(c *gin.Context, password string) bool {
	if len(password) < repo.MinPasswordLength || len(password) > repo.MaxPasswordLength {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("密码需 %d–%d 个字符", repo.MinPasswordLength, repo.MaxPasswordLength)})
		return false
	}
	return true
}

func (h *usersHandlers) createPlatformAccount(c *gin.Context) {
	if !h.platformAdminWrite(c) {
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 4096)
	var input struct {
		LoginName string `json:"loginName"`
		Password  string `json:"password"`
		Admin     bool   `json:"admin"`
	}
	if c.ShouldBindJSON(&input) != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid account request"})
		return
	}
	input.LoginName = strings.TrimSpace(input.LoginName)
	if !loginPattern.MatchString(input.LoginName) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "用户名需 2–64 位，以字母或数字开头，只能包含字母、数字、_ . -"})
		return
	}
	if !validAccountPassword(c, input.Password) {
		return
	}
	registerMu.Lock()
	u, err := h.repo.CreateLocalAccount(input.LoginName, input.Password, input.Admin)
	if err == nil {
		joinWelcomeProject(h.repo, u.ID)
	}
	registerMu.Unlock()
	if errors.Is(err, repo.ErrLoginTaken) {
		c.JSON(http.StatusConflict, gin.H{"error": "该用户名已被使用"})
		return
	}
	if err != nil {
		respond(c, err)
		return
	}
	c.JSON(http.StatusCreated, h.platformAccount(u.ID))
}

func (h *usersHandlers) platformAccount(userID int64) repo.PlatformAccount {
	accounts, _ := h.repo.ListPlatformAccounts()
	for _, account := range accounts {
		if account.UserID == userID {
			return account
		}
	}
	return repo.PlatformAccount{UserID: userID}
}

// resetPlatformAccountPassword sets a new password for a password account.
// Accounts that sign in through GitHub or a gateway are left alone, so this
// cannot add a password login to them.
func (h *usersHandlers) resetPlatformAccountPassword(c *gin.Context) {
	if !h.platformAdminWrite(c) {
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "valid user id is required"})
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 4096)
	var input struct {
		Password string `json:"password"`
	}
	if c.ShouldBindJSON(&input) != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid password request"})
		return
	}
	if !validAccountPassword(c, input.Password) {
		return
	}
	account := h.platformAccount(id)
	if account.LoginName == "" {
		c.JSON(http.StatusNotFound, gin.H{"error": "账号不存在"})
		return
	}
	if !account.Password {
		c.JSON(http.StatusBadRequest, gin.H{"error": "该账号不是用户名密码账号（例如通过 GitHub 登录），不能在这里设置密码"})
		return
	}
	if err := h.repo.SetLocalPassword(id, input.Password); err != nil {
		respond(c, err)
		return
	}
	c.JSON(http.StatusOK, account)
}
