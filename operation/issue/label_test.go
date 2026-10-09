// SPDX-License-Identifier: GPL-3.0-or-later

package issue

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/flag"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forgejo"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/mark3labs/mcp-go/mcp"
)

// newLabelBackend builds a test server where mux routes specific label paths.
// Records all non-version requests.
func newLabelBackend(t *testing.T, muxFn func(*http.ServeMux)) (*httptest.Server, *[]recordedReq) {
	t.Helper()
	records := make([]recordedReq, 0, 4)
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/version", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"version":"11.0.0+gitea-1.22.0"}`))
	})
	muxFn(mux)
	// Catch-all recorder
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		records = append(records, recordedReq{
			method:  r.Method,
			path:    r.URL.Path,
			escaped: r.URL.EscapedPath(),
			rawBody: body,
		})
		w.WriteHeader(http.StatusNotFound)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	flag.URL = srv.URL
	flag.Token = "tkn"
	flag.UserAgent = "test"
	c, err := forgejo_sdk.NewClient(srv.URL,
		forgejo_sdk.SetToken("tkn"),
		forgejo_sdk.SetUserAgent("test"),
	)
	if err != nil {
		t.Fatalf("sdk client: %v", err)
	}
	forgejo.SetClientForTesting(c)
	return srv, &records
}

func labelHandler(t *testing.T, method, path string, status int, body any) func(*http.ServeMux) {
	t.Helper()
	return func(mux *http.ServeMux) {
		mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
			if r.Method != method {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(status)
			if body != nil {
				json.NewEncoder(w).Encode(body)
			}
		})
	}
}

// ---- normalizeColor ----

func TestNormalizeColor(t *testing.T) {
	cases := []struct {
		in  string
		out string
		err bool
	}{
		{"0088ff", "#0088ff", false},
		{"#0088ff", "#0088ff", false},
		{"#0088FF", "#0088ff", false},
		{"0088FF", "#0088ff", false},
		{"abc", "", true},    // 3-digit rejected
		{"#abc", "", true},   // 3-digit rejected
		{"gggggg", "", true}, // invalid hex
		{"", "", true},
	}
	for _, c := range cases {
		got, err := normalizeColor(c.in)
		if c.err {
			if err == nil {
				t.Errorf("normalizeColor(%q): want error, got %q", c.in, got)
			}
		} else {
			if err != nil {
				t.Errorf("normalizeColor(%q): unexpected error: %v", c.in, err)
			} else if got != c.out {
				t.Errorf("normalizeColor(%q): got %q, want %q", c.in, got, c.out)
			}
		}
	}
}

// ---- create_repo_label ----

func TestCreateRepoLabelFn_HappyPath(t *testing.T) {
	label := forgejo_sdk.Label{ID: 1, Name: "bug", Color: "0088ff", Description: "a bug"}
	newLabelBackend(t, labelHandler(t, http.MethodPost, "/api/v1/repos/owner/repo/labels", http.StatusCreated, label))

	res, err := CreateRepoLabelFn(context.Background(), makeReq(map[string]any{
		"owner": "owner", "repo": "repo", "name": "bug", "color": "0088ff",
	}))
	if err != nil || res == nil || res.IsError {
		t.Fatalf("unexpected error: err=%v res=%+v", err, res)
	}
}

func TestCreateRepoLabelFn_MissingHash(t *testing.T) {
	label := forgejo_sdk.Label{ID: 2, Name: "bug", Color: "aabbcc"}
	newLabelBackend(t, labelHandler(t, http.MethodPost, "/api/v1/repos/owner/repo/labels", http.StatusCreated, label))

	res, err := CreateRepoLabelFn(context.Background(), makeReq(map[string]any{
		"owner": "owner", "repo": "repo", "name": "bug", "color": "aabbcc",
	}))
	if err != nil || res == nil || res.IsError {
		t.Fatalf("should succeed normalizing aabbcc: err=%v", err)
	}
}

func TestCreateRepoLabelFn_InvalidColor(t *testing.T) {
	newLabelBackend(t, func(_ *http.ServeMux) {})
	_, err := CreateRepoLabelFn(context.Background(), makeReq(map[string]any{
		"owner": "owner", "repo": "repo", "name": "bug", "color": "gg00ff",
	}))
	if err == nil {
		t.Fatal("expected error for invalid color")
	}
}

// ---- edit_repo_label ----

func TestEditRepoLabelFn_PartialPatch(t *testing.T) {
	_, records := newPatchBackend(t, `{"id":1,"name":"bug","color":"#0088ff"}`)

	res, err := EditRepoLabelFn(context.Background(), makeReq(map[string]any{
		"owner": "owner", "repo": "repo", "id": float64(1), "name": "bug-renamed",
	}))
	if err != nil || res == nil || res.IsError {
		t.Fatalf("edit failed: err=%v", err)
	}
	if len(*records) == 0 {
		t.Fatal("no request recorded")
	}
	// name should be set; color should be null (SDK marshals nil *string as null, not omitted).
	var body map[string]any
	_ = json.Unmarshal((*records)[0].rawBody, &body)
	if body["name"] == nil {
		t.Error("name should be set in PATCH body")
	}
	if _, ok := body["color"]; ok {
		t.Errorf("color should be omitted when not provided, got %v", body["color"])
	}
	if _, ok := body["exclusive"]; ok {
		t.Errorf("exclusive should be omitted on name-only PATCH, got %v", body["exclusive"])
	}
}

func TestEditRepoLabelFn_EmptyReject(t *testing.T) {
	_, records := newPatchBackend(t, `{}`)
	_, err := EditRepoLabelFn(context.Background(), makeReq(map[string]any{
		"owner": "owner", "repo": "repo", "id": float64(1),
	}))
	if err == nil {
		t.Fatal("expected error when no fields provided")
	}
	for _, rec := range *records {
		if rec.method == http.MethodPatch {
			t.Errorf("PATCH must not be sent for empty edit, got %+v", rec)
		}
	}
}

// ---- delete_repo_label ----

func TestDeleteRepoLabelFn_UnusedSuccess(t *testing.T) {
	muxSetup := func(mux *http.ServeMux) {
		// GET label
		mux.HandleFunc("/api/v1/repos/owner/repo/labels/1", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			switch r.Method {
			case http.MethodGet:
				json.NewEncoder(w).Encode(forgejo_sdk.Label{ID: 1, Name: "unused"})
			case http.MethodDelete:
				w.WriteHeader(http.StatusNoContent)
			}
		})
		// Issues list with X-Total-Count: 0
		mux.HandleFunc("/api/v1/repos/owner/repo/issues", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("X-Total-Count", "0")
			_, _ = w.Write([]byte("[]"))
		})
	}
	newLabelBackend(t, muxSetup)

	res, err := DeleteRepoLabelFn(context.Background(), makeReq(map[string]any{
		"owner": "owner", "repo": "repo", "id": float64(1),
	}))
	if err != nil || res == nil || res.IsError {
		t.Fatalf("delete unused label failed: err=%v", err)
	}
}

func TestDeleteRepoLabelFn_InUseRefused(t *testing.T) {
	muxSetup := func(mux *http.ServeMux) {
		mux.HandleFunc("/api/v1/repos/owner/repo/labels/2", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(forgejo_sdk.Label{ID: 2, Name: "inuse"})
		})
		mux.HandleFunc("/api/v1/repos/owner/repo/issues", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("X-Total-Count", "3")
			_, _ = w.Write([]byte("[]"))
		})
	}
	newLabelBackend(t, muxSetup)

	_, err := DeleteRepoLabelFn(context.Background(), makeReq(map[string]any{
		"owner": "owner", "repo": "repo", "id": float64(2),
	}))
	if err == nil || !strings.Contains(err.Error(), "is used by") {
		t.Fatalf("expected in-use error, got: %v", err)
	}
}

func TestDeleteRepoLabelFn_InUseForceSuccess(t *testing.T) {
	muxSetup := func(mux *http.ServeMux) {
		mux.HandleFunc("/api/v1/repos/owner/repo/labels/3", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			if r.Method == http.MethodDelete {
				w.WriteHeader(http.StatusNoContent)
			}
		})
	}
	newLabelBackend(t, muxSetup)

	res, err := DeleteRepoLabelFn(context.Background(), makeReq(map[string]any{
		"owner": "owner", "repo": "repo", "id": float64(3), "delete_mode": "force",
	}))
	if err != nil || res == nil || res.IsError {
		t.Fatalf("force delete failed: err=%v", err)
	}
}

func TestDeleteRepoLabelFn_404(t *testing.T) {
	muxSetup := func(mux *http.ServeMux) {
		mux.HandleFunc("/api/v1/repos/owner/repo/labels/99", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{}`))
		})
	}
	newLabelBackend(t, muxSetup)

	_, err := DeleteRepoLabelFn(context.Background(), makeReq(map[string]any{
		"owner": "owner", "repo": "repo", "id": float64(99), "delete_mode": "force",
	}))
	if err == nil {
		t.Fatal("expected error for 404")
	}
}

// ---- get_repo_label ----

func TestGetRepoLabelFn_HappyPath(t *testing.T) {
	label := forgejo_sdk.Label{ID: 5, Name: "priority", Color: "#ff0000"}
	newLabelBackend(t, labelHandler(t, http.MethodGet, "/api/v1/repos/owner/repo/labels/5", http.StatusOK, label))

	res, err := GetRepoLabelFn(context.Background(), makeReq(map[string]any{
		"owner": "owner", "repo": "repo", "id": float64(5),
	}))
	if err != nil || res == nil || res.IsError {
		t.Fatalf("get label failed: err=%v", err)
	}
	out := textOf(res)
	if !strings.Contains(out, `"exclusive":false`) && !strings.Contains(out, `"exclusive": false`) {
		t.Errorf("expected exclusive false on get, got %s", out)
	}
	if !strings.Contains(out, `"is_archived":false`) && !strings.Contains(out, `"is_archived": false`) {
		t.Errorf("expected is_archived false on get, got %s", out)
	}
}

func TestGetRepoLabelFn_404(t *testing.T) {
	newLabelBackend(t, func(mux *http.ServeMux) {
		mux.HandleFunc("/api/v1/repos/owner/repo/labels/404", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{}`))
		})
	})
	_, err := GetRepoLabelFn(context.Background(), makeReq(map[string]any{
		"owner": "owner", "repo": "repo", "id": float64(404),
	}))
	if err == nil {
		t.Fatal("expected error for missing label")
	}
}

// ---- label resources parse helpers (smoke tests) ----

func TestParseLabel_InvalidID(t *testing.T) {
	req := mcp.ReadResourceRequest{Params: mcp.ReadResourceParams{URI: "forgejo://repo/owner/repo/label/abc"}}
	_, err := repoLabelResourceHandler(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for non-numeric id")
	}
}

func TestRepoLabelsResource_HappyPath(t *testing.T) {
	labels := []forgejo_sdk.Label{
		{ID: 1, Name: "bug", Color: "#ff0000"},
		{ID: 2, Name: "feature", Color: "#0088ff"},
	}
	muxSetup := func(mux *http.ServeMux) {
		mux.HandleFunc("/api/v1/repos/owner/repo/labels", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(labels)
		})
	}
	newLabelBackend(t, muxSetup)

	req := mcp.ReadResourceRequest{Params: mcp.ReadResourceParams{
		URI: "forgejo://repo/owner/repo/labels",
	}}
	contents, err := repoLabelsResourceHandler(context.Background(), req)
	if err != nil {
		t.Fatalf("resource handler error: %v", err)
	}
	if len(contents) == 0 {
		t.Fatal("expected contents")
	}
	text := contents[0].(mcp.TextResourceContents).Text
	if !strings.Contains(text, `"exclusive"`) {
		t.Errorf("list resource JSON missing exclusive, got %s", text)
	}
	if !strings.Contains(text, `"is_archived"`) {
		t.Errorf("list resource JSON missing is_archived, got %s", text)
	}
}

func TestRepoLabelsResource_OverCap(t *testing.T) {
	// Build 31 labels (> EmbeddedListCap=30) to trigger truncation.
	labels := make([]forgejo_sdk.Label, 31)
	for i := range labels {
		labels[i] = forgejo_sdk.Label{ID: int64(i + 1), Name: fmt.Sprintf("label-%d", i+1)}
	}
	muxSetup := func(mux *http.ServeMux) {
		mux.HandleFunc("/api/v1/repos/owner/repo/labels", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(labels)
		})
	}
	newLabelBackend(t, muxSetup)

	req := mcp.ReadResourceRequest{Params: mcp.ReadResourceParams{
		URI: "forgejo://repo/owner/repo/labels",
	}}
	contents, err := repoLabelsResourceHandler(context.Background(), req)
	if err != nil {
		t.Fatalf("resource handler error: %v", err)
	}
	var payload labelsListPayload
	tc := contents[0].(mcp.TextResourceContents)
	_ = json.Unmarshal([]byte(tc.Text), &payload)
	if !payload.Truncated {
		t.Error("expected truncated=true for 31 labels")
	}
	if payload.ListTool != ListRepoLabelsToolName {
		t.Errorf("expected list_tool=%q got %q", ListRepoLabelsToolName, payload.ListTool)
	}
}

func TestRepoLabelsResource_404(t *testing.T) {
	muxSetup := func(mux *http.ServeMux) {
		mux.HandleFunc("/api/v1/repos/owner/repo/labels", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{}`))
		})
	}
	newLabelBackend(t, muxSetup)

	req := mcp.ReadResourceRequest{Params: mcp.ReadResourceParams{URI: "forgejo://repo/owner/repo/labels"}}
	_, err := repoLabelsResourceHandler(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for 404")
	}
}

func TestIsScopedLabelName(t *testing.T) {
	cases := []struct {
		name string
		ok   bool
	}{
		{"kind/bug", true},
		{"scope/sub/item", true},
		{"bug", false},
		{"/bug", false},
		{"kind/", false},
		{"a/b/", false},
		{"", false},
	}
	for _, c := range cases {
		if got := isScopedLabelName(c.name); got != c.ok {
			t.Errorf("isScopedLabelName(%q)=%v, want %v", c.name, got, c.ok)
		}
	}
}

func labelJSON(exclusive, archived bool) string {
	return fmt.Sprintf(`{"id":1,"name":"kind/bug","color":"#0088ff","description":"","exclusive":%t,"is_archived":%t}`, exclusive, archived)
}

func assertJSONBool(t *testing.T, raw []byte, key string, want bool) {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Errorf("body JSON: %v (raw=%s)", err, raw)
		return
	}
	v, ok := body[key]
	if !ok {
		t.Errorf("%s missing from body %v", key, body)
		return
	}
	b, ok := v.(bool)
	if !ok || b != want {
		t.Errorf("%s: got %v (%T), want %v", key, v, v, want)
	}
}

func assertJSONKeyAbsent(t *testing.T, raw []byte, key string) {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Errorf("body JSON: %v (raw=%s)", err, raw)
		return
	}
	if _, ok := body[key]; ok {
		t.Errorf("%s must be omitted, body=%v", key, body)
	}
}

func TestCreateRepoLabelFn_OmitsExclusive(t *testing.T) {
	var postBody []byte
	newLabelBackend(t, func(mux *http.ServeMux) {
		mux.HandleFunc("/api/v1/repos/owner/repo/labels", func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}
			postBody, _ = io.ReadAll(r.Body)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(labelJSON(false, false)))
		})
	})
	res, err := CreateRepoLabelFn(context.Background(), makeReq(map[string]any{
		"owner": "owner", "repo": "repo", "name": "bug", "color": "0088ff",
	}))
	if err != nil || res == nil || res.IsError {
		t.Fatalf("create failed: err=%v res=%+v", err, res)
	}
	assertJSONKeyAbsent(t, postBody, "exclusive")
	assertJSONKeyAbsent(t, postBody, "is_archived")
}

func TestCreateRepoLabelFn_ExclusiveTrueScoped(t *testing.T) {
	var postBody []byte
	newLabelBackend(t, func(mux *http.ServeMux) {
		mux.HandleFunc("/api/v1/repos/owner/repo/labels", func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}
			postBody, _ = io.ReadAll(r.Body)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(labelJSON(true, false)))
		})
	})
	res, err := CreateRepoLabelFn(context.Background(), makeReq(map[string]any{
		"owner": "owner", "repo": "repo", "name": "kind/bug", "color": "0088ff", "exclusive": true,
	}))
	if err != nil || res == nil || res.IsError {
		t.Fatalf("create failed: err=%v res=%+v", err, res)
	}
	assertJSONBool(t, postBody, "exclusive", true)
	out := textOf(res)
	if !strings.Contains(out, `"exclusive":true`) && !strings.Contains(out, `"exclusive": true`) {
		t.Errorf("result missing exclusive true, got %s", out)
	}
}

func TestCreateRepoLabelFn_ExclusiveTrueUnscopedNoHTTP(t *testing.T) {
	for _, name := range []string{"bug", "/bug", "kind/"} {
		t.Run(name, func(t *testing.T) {
			_, records := newLabelBackend(t, func(_ *http.ServeMux) {})
			_, err := CreateRepoLabelFn(context.Background(), makeReq(map[string]any{
				"owner": "owner", "repo": "repo", "name": name, "color": "0088ff", "exclusive": true,
			}))
			if err == nil {
				t.Fatal("expected error")
			}
			if len(*records) != 0 {
				t.Errorf("HTTP must not be sent, got %d requests: %+v", len(*records), *records)
			}
		})
	}
}

func TestCreateOrgLabelFn_ExclusiveTrueScoped(t *testing.T) {
	var postBody []byte
	newLabelBackend(t, func(mux *http.ServeMux) {
		mux.HandleFunc("/api/v1/orgs/org/labels", func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}
			postBody, _ = io.ReadAll(r.Body)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(labelJSON(true, false)))
		})
	})
	res, err := CreateOrgLabelFn(context.Background(), makeReq(map[string]any{
		"org": "org", "name": "kind/bug", "color": "0088ff", "exclusive": true,
	}))
	if err != nil || res == nil || res.IsError {
		t.Fatalf("create failed: err=%v res=%+v", err, res)
	}
	assertJSONBool(t, postBody, "exclusive", true)
}

func TestEditRepoLabelFn_ColorOnlyOmitsFlags(t *testing.T) {
	var patchBody []byte
	newLabelBackend(t, func(mux *http.ServeMux) {
		mux.HandleFunc("/api/v1/repos/owner/repo/labels/1", func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPatch {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}
			patchBody, _ = io.ReadAll(r.Body)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(labelJSON(false, false)))
		})
	})
	res, err := EditRepoLabelFn(context.Background(), makeReq(map[string]any{
		"owner": "owner", "repo": "repo", "id": float64(1), "color": "aabbcc",
	}))
	if err != nil || res == nil || res.IsError {
		t.Fatalf("edit failed: err=%v res=%+v", err, res)
	}
	assertJSONKeyAbsent(t, patchBody, "exclusive")
	assertJSONKeyAbsent(t, patchBody, "is_archived")
	var body map[string]any
	_ = json.Unmarshal(patchBody, &body)
	if body["color"] != "#aabbcc" {
		t.Errorf("color: got %v", body["color"])
	}
}

func TestEditRepoLabelFn_ExclusiveFalseSent(t *testing.T) {
	var patchBody []byte
	newLabelBackend(t, func(mux *http.ServeMux) {
		mux.HandleFunc("/api/v1/repos/owner/repo/labels/1", func(w http.ResponseWriter, r *http.Request) {
			patchBody, _ = io.ReadAll(r.Body)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(labelJSON(false, false)))
		})
	})
	if _, err := EditRepoLabelFn(context.Background(), makeReq(map[string]any{
		"owner": "owner", "repo": "repo", "id": float64(1), "exclusive": false,
	})); err != nil {
		t.Fatalf("edit failed: %v", err)
	}
	assertJSONBool(t, patchBody, "exclusive", false)
	assertJSONKeyAbsent(t, patchBody, "is_archived")
}

func TestEditRepoLabelFn_ArchivedFalseSent(t *testing.T) {
	var patchBody []byte
	newLabelBackend(t, func(mux *http.ServeMux) {
		mux.HandleFunc("/api/v1/repos/owner/repo/labels/1", func(w http.ResponseWriter, r *http.Request) {
			patchBody, _ = io.ReadAll(r.Body)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(labelJSON(false, false)))
		})
	})
	if _, err := EditRepoLabelFn(context.Background(), makeReq(map[string]any{
		"owner": "owner", "repo": "repo", "id": float64(1), "is_archived": false,
	})); err != nil {
		t.Fatalf("edit failed: %v", err)
	}
	assertJSONBool(t, patchBody, "is_archived", false)
	assertJSONKeyAbsent(t, patchBody, "exclusive")
}

func TestEditRepoLabelFn_ExclusiveOnlyAccepted(t *testing.T) {
	var patchBody []byte
	newLabelBackend(t, func(mux *http.ServeMux) {
		mux.HandleFunc("/api/v1/repos/owner/repo/labels/1", func(w http.ResponseWriter, r *http.Request) {
			patchBody, _ = io.ReadAll(r.Body)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(labelJSON(true, false)))
		})
	})
	if _, err := EditRepoLabelFn(context.Background(), makeReq(map[string]any{
		"owner": "owner", "repo": "repo", "id": float64(1), "exclusive": true,
	})); err != nil {
		t.Fatalf("edit failed: %v", err)
	}
	assertJSONBool(t, patchBody, "exclusive", true)
	assertJSONKeyAbsent(t, patchBody, "name")
	assertJSONKeyAbsent(t, patchBody, "color")
}

func TestEditRepoLabelFn_ArchivedOnlyAccepted(t *testing.T) {
	var patchBody []byte
	newLabelBackend(t, func(mux *http.ServeMux) {
		mux.HandleFunc("/api/v1/repos/owner/repo/labels/1", func(w http.ResponseWriter, r *http.Request) {
			patchBody, _ = io.ReadAll(r.Body)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(labelJSON(false, true)))
		})
	})
	if _, err := EditRepoLabelFn(context.Background(), makeReq(map[string]any{
		"owner": "owner", "repo": "repo", "id": float64(1), "is_archived": true,
	})); err != nil {
		t.Fatalf("edit failed: %v", err)
	}
	assertJSONBool(t, patchBody, "is_archived", true)
}

func TestEditOrgLabelFn_ExclusiveFalseSent(t *testing.T) {
	var patchBody []byte
	newLabelBackend(t, func(mux *http.ServeMux) {
		mux.HandleFunc("/api/v1/orgs/org/labels/1", func(w http.ResponseWriter, r *http.Request) {
			patchBody, _ = io.ReadAll(r.Body)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(labelJSON(false, false)))
		})
	})
	if _, err := EditOrgLabelFn(context.Background(), makeReq(map[string]any{
		"org": "org", "id": float64(1), "exclusive": false,
	})); err != nil {
		t.Fatalf("edit failed: %v", err)
	}
	assertJSONBool(t, patchBody, "exclusive", false)
}

func TestGetRepoLabelFn_OwnerSlashEscaped(t *testing.T) {
	_, records := newLabelBackend(t, func(_ *http.ServeMux) {})
	_, err := GetRepoLabelFn(context.Background(), makeReq(map[string]any{
		"owner": "acme/org", "repo": "r", "id": float64(1),
	}))
	if err == nil {
		t.Fatal("expected error from missing label")
	}
	if len(*records) == 0 {
		t.Fatal("expected HTTP request")
		return
	}
	escaped := (*records)[0].escaped
	if !strings.Contains(escaped, "acme%2Forg") {
		t.Errorf("escaped path %q should contain acme%%2Forg", escaped)
	}
}

func TestRepoLabelResource_IncludesExclusive(t *testing.T) {
	newLabelBackend(t, labelHandler(t, http.MethodGet, "/api/v1/repos/owner/repo/labels/1", http.StatusOK, map[string]any{
		"id": 1, "name": "kind/bug", "color": "#ff0000", "exclusive": true, "is_archived": false,
	}))
	contents, err := repoLabelResourceHandler(context.Background(), mcp.ReadResourceRequest{
		Params: mcp.ReadResourceParams{URI: "forgejo://repo/owner/repo/label/1"},
	})
	if err != nil {
		t.Fatalf("resource: %v", err)
	}
	text := contents[0].(mcp.TextResourceContents).Text
	if !strings.Contains(text, `"exclusive":true`) && !strings.Contains(text, `"exclusive": true`) {
		t.Errorf("missing exclusive true, got %s", text)
	}
	if !strings.Contains(text, `"is_archived":false`) && !strings.Contains(text, `"is_archived": false`) {
		t.Errorf("missing is_archived false, got %s", text)
	}
}
