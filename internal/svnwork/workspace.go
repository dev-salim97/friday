package svnwork

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"unicode"

	"agentflow/internal/procwatch"
)

// ValidateRemoteURL accepts an exact repository directory URL. SVN helper
// transports such as svn+ssh and local filesystem URLs are excluded.
func ValidateRemoteURL(raw string) (remote, name string, err error) {
	raw = strings.TrimSpace(raw)
	u, err := url.Parse(raw)
	if err != nil || u.Hostname() == "" || u.User != nil ||
		strings.ContainsAny(raw, "\\?#") || strings.IndexFunc(raw, unicode.IsSpace) >= 0 ||
		(u.Scheme != "https" && u.Scheme != "http" && u.Scheme != "svn") {
		return "", "", errors.New("SVN URL must use https://, http:// or svn:// without credentials, a query or a fragment")
	}
	repositoryPath := strings.TrimSuffix(u.Path, "/")
	for _, part := range strings.Split(strings.TrimPrefix(repositoryPath, "/"), "/") {
		if part == "" || part == "." || part == ".." || strings.ContainsRune(part, '\\') ||
			strings.IndexFunc(part, unicode.IsControl) >= 0 {
			return "", "", errors.New("SVN URL must specify a valid repository path")
		}
	}
	u.Host = strings.ToLower(u.Host)
	u.Path = repositoryPath
	u.RawPath = strings.TrimSuffix(u.RawPath, "/")
	return u.String(), path.Base(repositoryPath), nil
}

// Prepare checks out remote into workspace. Reopening the same working copy
// keeps its checked-out revision and local changes.
func Prepare(ctx context.Context, workspace, remote string, options procwatch.Options) error {
	remote, _, err := ValidateRemoteURL(remote)
	if err != nil {
		return err
	}
	client, err := exec.LookPath("svn")
	if err != nil {
		return fmt.Errorf("install the Subversion (svn) client on the execution host: %w", err)
	}
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		return err
	}
	config, err := os.MkdirTemp("", "friday-svn-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(config)
	flags := []string{"--non-interactive", "--no-auth-cache", "--config-dir", config}
	command := func(ctx context.Context, args ...string) *exec.Cmd {
		cmd := exec.CommandContext(ctx, client, append(args, flags...)...)
		cmd.Dir = workspace
		return cmd
	}
	adminDir := filepath.Join(workspace, ".svn")
	readyPath := filepath.Join(adminDir, "friday-ready")
	if _, err := os.Stat(adminDir); err == nil {
		out, err := command(ctx, "info", "--show-item", "url", ".").Output()
		if err != nil {
			return fmt.Errorf("read SVN working copy URL: %w", err)
		}
		current, _, err := ValidateRemoteURL(string(out))
		if err != nil || current != remote {
			return errors.New("SVN working copy belongs to a different source URL")
		}
		if _, err := os.Stat(readyPath); err == nil {
			return nil
		} else if !os.IsNotExist(err) {
			return err
		}
		if out, err := command(ctx, "cleanup", ".").CombinedOutput(); err != nil {
			return fmt.Errorf("svn cleanup: %w: %s", err, strings.TrimSpace(string(out)))
		}
	} else if !os.IsNotExist(err) {
		return err
	}

	tail, err := procwatch.Run(ctx, func(ctx context.Context) *exec.Cmd {
		// An empty peg suffix makes a literal @ in the URL unambiguous.
		return command(ctx, "checkout", "--ignore-externals", remote+"@", ".")
	}, options)
	if err != nil {
		return fmt.Errorf("svn checkout: %w: %s", err, strings.TrimSpace(tail))
	}
	return os.WriteFile(readyPath, nil, 0o600)
}
