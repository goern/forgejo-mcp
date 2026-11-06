// SPDX-License-Identifier: GPL-3.0-or-later

package pull

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/flag"
)

const mergeRequiredBody = `{"message":"[Do]: Required","url":"https://forge.example/api/swagger"}`

// mergeBackend answers the merge endpoint with status and body, recording each
// request so a test can assert what was sent, or that nothing was.
type mergeBackend struct {
	paths  []string
	bodies []map[string]any
}

func newMergeBackend(t *testing.T, status int, body string) *mergeBackend {
	t.Helper()
	b := &mergeBackend{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var got map[string]any
		_ = json.NewDecoder(r.Body).Decode(&got)
		b.paths = append(b.paths, r.Method+" "+r.URL.Path)
		b.bodies = append(b.bodies, got)
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	oldURL, oldToken := flag.URL, flag.Token
	t.Cleanup(func() { flag.URL, flag.Token = oldURL, oldToken })
	flag.URL, flag.Token = srv.URL, "tkn"
	return b
}

func mergeArgs(extra map[string]any) map[string]any {
	args := map[string]any{"owner": "o", "repo": "r", "index": float64(7)}
	for k, v := range extra {
		args[k] = v
	}
	return args
}

func TestMergePullRequest_MissingStyleSendsNothing(t *testing.T) {
	b := newMergeBackend(t, http.StatusOK, "")

	_, err := MergePullRequestFn(context.Background(), makeReq(mergeArgs(nil)))

	if err == nil || !strings.Contains(err.Error(), "style is required") {
		t.Fatalf("err = %v, want a style-is-required error", err)
	}
	if len(b.paths) != 0 {
		t.Errorf("sent %v, want no request", b.paths)
	}
}

func TestMergePullRequest_MergeStyleParameterIsNamed(t *testing.T) {
	b := newMergeBackend(t, http.StatusOK, "")

	_, err := MergePullRequestFn(context.Background(), makeReq(mergeArgs(map[string]any{"merge_style": "merge"})))

	if err == nil || !strings.Contains(err.Error(), "unknown parameter merge_style, use style") {
		t.Fatalf("err = %v, want it to name merge_style", err)
	}
	if len(b.paths) != 0 {
		t.Errorf("sent %v, want no request", b.paths)
	}
}

func TestMergePullRequest_SendsStyleAsDo(t *testing.T) {
	b := newMergeBackend(t, http.StatusOK, "")

	res, err := MergePullRequestFn(context.Background(), makeReq(mergeArgs(map[string]any{
		"style": "squash", "title": "t", "message": "m", "delete_branch_after_merge": true,
	})))
	if err != nil {
		t.Fatalf("merge: %v", err)
	}

	if got := textOf(res); got != "Pull request merged successfully" {
		t.Errorf("result = %q", got)
	}
	if len(b.paths) != 1 || b.paths[0] != "POST /api/v1/repos/o/r/pulls/7/merge" {
		t.Fatalf("paths = %v", b.paths)
	}
	sent := b.bodies[0]
	if sent["Do"] != "squash" || sent["MergeTitleField"] != "t" || sent["MergeMessageField"] != "m" ||
		sent["delete_branch_after_merge"] != true {
		t.Errorf("body = %v", sent)
	}
}

func TestMergePullRequest_RefusalCarriesForgejoReason(t *testing.T) {
	newMergeBackend(t, http.StatusUnprocessableEntity, mergeRequiredBody)

	_, err := MergePullRequestFn(context.Background(), makeReq(mergeArgs(map[string]any{"style": "nonsense"})))

	if err == nil || !strings.Contains(err.Error(), "[Do]: Required") {
		t.Fatalf("err = %v, want Forgejo's message", err)
	}
}

func TestMergePullRequest_RefusedScheduleIsNotReportedScheduled(t *testing.T) {
	newMergeBackend(t, http.StatusUnprocessableEntity, mergeRequiredBody)

	res, err := MergePullRequestFn(context.Background(), makeReq(mergeArgs(map[string]any{
		"style": "merge", "merge_when_checks_succeed": true,
	})))

	if err == nil {
		t.Fatalf("got result %q, want an error", textOf(res))
	}
}

func TestMergePullRequest_ScheduledMerge(t *testing.T) {
	newMergeBackend(t, http.StatusCreated, "")

	res, err := MergePullRequestFn(context.Background(), makeReq(mergeArgs(map[string]any{
		"style": "merge", "merge_when_checks_succeed": true,
	})))
	if err != nil {
		t.Fatalf("merge: %v", err)
	}

	if got := textOf(res); got != "Pull request scheduled to merge when all checks succeed" {
		t.Errorf("result = %q", got)
	}
}
