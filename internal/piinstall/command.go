package piinstall

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// maxCapturedOutput bounds captured child output so a noisy or secret-bearing
// process cannot flood the report. Only a bounded prefix is retained.
const maxCapturedOutput = 64 * 1024

// maxDownloadRedirects bounds how many redirect hops a download may follow,
// regardless of any caller-supplied redirect policy.
const maxDownloadRedirects = 10

// ExecRunner is the production CommandRunner. It never edits PATH or the user's
// shell; callers pass a fully scoped Env in each CommandSpec when needed.
type ExecRunner struct {
	Timeout time.Duration
}

// LookPath resolves a conventional executable name on the current PATH.
func (r ExecRunner) LookPath(name string) (string, error) {
	return exec.LookPath(name)
}

// Run executes one bounded child process and captures its output prefix.
func (r ExecRunner) Run(ctx context.Context, spec CommandSpec) CommandResult {
	timeout := r.Timeout
	if timeout <= 0 {
		timeout = 2 * time.Minute
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(runCtx, spec.Name, spec.Args...)
	if spec.Dir != "" {
		cmd.Dir = spec.Dir
	}
	if spec.Env != nil {
		cmd.Env = spec.Env
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &boundedWriter{buf: &stdout}
	cmd.Stderr = &boundedWriter{buf: &stderr}

	result := CommandResult{}
	err := cmd.Run()
	result.Stdout = stdout.String()
	result.Stderr = stderr.String()
	result.Err = err
	if err != nil {
		var exitErr *exec.ExitError
		if ok := asExitError(err, &exitErr); ok {
			result.ExitCode = exitErr.ExitCode()
		} else {
			result.ExitCode = -1
		}
	}
	return result
}

func asExitError(err error, target **exec.ExitError) bool {
	exitErr, ok := err.(*exec.ExitError)
	if ok {
		*target = exitErr
	}
	return ok
}

type boundedWriter struct {
	buf *bytes.Buffer
}

func (w *boundedWriter) Write(p []byte) (int, error) {
	if w.buf.Len() >= maxCapturedOutput {
		return len(p), nil
	}
	remaining := maxCapturedOutput - w.buf.Len()
	if len(p) > remaining {
		w.buf.Write(p[:remaining])
		return len(p), nil
	}
	w.buf.Write(p)
	return len(p), nil
}

// HTTPDownloader downloads an archive over https with a bounded body. It uses
// no curl/tar/xz binaries and no non-stdlib dependency.
type HTTPDownloader struct {
	Client  *http.Client
	Timeout time.Duration
}

// Download streams rawURL into destPath without following a non-https scheme,
// rejecting non-200 responses and bodies larger than maxBytes.
func (d HTTPDownloader) Download(ctx context.Context, rawURL, destPath string, maxBytes int64) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("piinstall: parse download url: %w", err)
	}
	if parsed.Scheme != "https" {
		return fmt.Errorf("piinstall: refusing non-https download scheme %q", parsed.Scheme)
	}

	timeout := d.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Minute
	}
	client := d.Client
	if client == nil {
		client = &http.Client{Timeout: timeout}
	}
	// Enforce https on every redirect hop without mutating the caller's shared
	// client or dropping the caller's redirect policy. The pinned SHA-256 still
	// protects bytes; this restriction is about transport only.
	secured := *client
	callerRedirect := client.CheckRedirect
	secured.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if req.URL.Scheme != "https" {
			return fmt.Errorf("piinstall: refusing non-https redirect to %s", req.URL.Redacted())
		}
		// Enforce the hop bound before consulting the caller policy so a
		// permissive callback cannot bypass it.
		if len(via) >= maxDownloadRedirects {
			return errors.New("piinstall: stopped after 10 redirects")
		}
		if callerRedirect != nil {
			return callerRedirect(req, via)
		}
		return nil
	}

	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, rawURL, nil)
	if err != nil {
		return fmt.Errorf("piinstall: build download request: %w", err)
	}
	resp, err := secured.Do(req)
	if err != nil {
		return fmt.Errorf("piinstall: download %s: %w", rawURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("piinstall: download %s: unexpected status %s", rawURL, resp.Status)
	}

	file, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("piinstall: create download staging file: %w", err)
	}
	written, copyErr := io.Copy(file, io.LimitReader(resp.Body, maxBytes+1))
	closeErr := file.Close()
	if copyErr != nil {
		return fmt.Errorf("piinstall: download body: %w", copyErr)
	}
	if closeErr != nil {
		return fmt.Errorf("piinstall: close download staging file: %w", closeErr)
	}
	if written > maxBytes {
		return fmt.Errorf("piinstall: download %s exceeds limit of %d bytes", rawURL, maxBytes)
	}
	return nil
}

func isAbsPath(path string) bool {
	return filepath.IsAbs(path)
}

func fileExists(path string) bool {
	info, err := os.Lstat(path)
	return err == nil && info.Mode().IsRegular()
}

func anyExists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

// joinToolPath builds a deterministic PATH from the given directories plus the
// inherited PATH. Empty entries and duplicates are dropped.
func joinToolPath(inherited string, dirs ...string) string {
	seen := map[string]bool{}
	ordered := make([]string, 0, len(dirs)+8)
	add := func(dir string) {
		dir = strings.TrimSpace(dir)
		if dir == "" || seen[dir] {
			return
		}
		seen[dir] = true
		ordered = append(ordered, dir)
	}
	for _, dir := range dirs {
		add(dir)
	}
	for _, dir := range filepath.SplitList(inherited) {
		add(dir)
	}
	return strings.Join(ordered, string(os.PathListSeparator))
}
