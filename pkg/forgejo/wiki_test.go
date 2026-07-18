// SPDX-License-Identifier: GPL-3.0-or-later

package forgejo

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"
)

func TestWikiPageNameRoundTripsEncodedSlash(t *testing.T) {
	_, captured := newCaptureServer(t, func(w http.ResponseWriter, r *http.Request, _ *capturedReq) {
		_, _ = w.Write([]byte(`{"title":"Child","sub_url":"Guides%2FSetup","content_base64":""}`))
	})
	if _, err := GetWikiPage(context.Background(), "o", "r", "Guides%2FSetup"); err != nil {
		t.Fatal(err)
	}
	if captured.path != "/api/v1/repos/o/r/wiki/page/Guides/Setup" && captured.path != "/api/v1/repos/o/r/wiki/page/Guides%2FSetup" {
		t.Fatalf("encoded slash did not round-trip: %q", captured.path)
	}
}

func TestListWikiPages404IsEmpty(t *testing.T) {
	newCaptureServer(t, func(w http.ResponseWriter, _ *http.Request, _ *capturedReq) { w.WriteHeader(http.StatusNotFound) })
	pages, err := ListWikiPages(context.Background(), "o", "r", 1, 31)
	if err != nil || len(pages) != 0 {
		t.Fatalf("pages=%v err=%v", pages, err)
	}
}

func TestWikiReadMethodsAndPaths(t *testing.T) {
	tests := []struct {
		name     string
		call     func() error
		path     string
		query    string
		response string
	}{
		{"list", func() error { _, err := ListWikiPages(context.Background(), "space owner", "repo", 2, 31); return err }, "/api/v1/repos/space%20owner/repo/wiki/pages", "page=2&limit=31", `[]`},
		{"get", func() error { _, err := GetWikiPage(context.Background(), "o", "r", "Guides%2FSetup"); return err }, "/api/v1/repos/o/r/wiki/page/Guides%2FSetup", "", `{"title":"T"}`},
		{"revisions", func() error {
			_, err := GetWikiPageRevisions(context.Background(), "o", "r", "Guides%2FSetup", 3, 11)
			return err
		}, "/api/v1/repos/o/r/wiki/revisions/Guides%2FSetup", "page=3&limit=11", `{"commits":[]}`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			newCaptureServer(t, func(w http.ResponseWriter, r *http.Request, _ *capturedReq) {
				if r.Method != http.MethodGet || r.URL.EscapedPath() != tc.path || r.URL.RawQuery != tc.query {
					t.Fatalf("request mismatch: %s %s?%s", r.Method, r.URL.EscapedPath(), r.URL.RawQuery)
				}
				_, _ = w.Write([]byte(tc.response))
			})
			if err := tc.call(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestWikiRevisions404IsError(t *testing.T) {
	newCaptureServer(t, func(w http.ResponseWriter, _ *http.Request, _ *capturedReq) { w.WriteHeader(http.StatusNotFound) })
	_, err := GetWikiPageRevisions(context.Background(), "o", "r", "missing", 1, 31)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestCreateWikiPageBody(t *testing.T) {
	_, captured := newCaptureServer(t, func(w http.ResponseWriter, _ *http.Request, _ *capturedReq) {
		_, _ = w.Write([]byte(`{"title":"T","sub_url":"T"}`))
	})
	if _, err := CreateWikiPage(context.Background(), "o", "r", "T", "aGVsbG8=", "create"); err != nil {
		t.Fatal(err)
	}
	var body map[string]string
	if err := json.Unmarshal(captured.body, &body); err != nil {
		t.Fatal(err)
	}
	if captured.method != http.MethodPost || body["content_base64"] != "aGVsbG8=" {
		t.Fatalf("request mismatch: method=%s body=%v", captured.method, body)
	}
}

func TestEditWikiPageMethodPathAndBody(t *testing.T) {
	_, captured := newCaptureServer(t, func(w http.ResponseWriter, r *http.Request, _ *capturedReq) {
		_, _ = w.Write([]byte(`{"title":"New","sub_url":"New"}`))
	})
	if _, err := EditWikiPage(context.Background(), "o", "r", "Old%2FPage", "New", "bmV3", "edit"); err != nil {
		t.Fatal(err)
	}
	var body map[string]string
	if err := json.Unmarshal(captured.body, &body); err != nil {
		t.Fatal(err)
	}
	if captured.method != http.MethodPatch || captured.path != "/api/v1/repos/o/r/wiki/page/Old/Page" || body["title"] != "New" || body["content_base64"] != "bmV3" || body["message"] != "edit" {
		t.Fatalf("request mismatch: method=%s path=%s body=%v", captured.method, captured.path, body)
	}
}

func TestDeleteWikiPageAcceptsAny2xx(t *testing.T) {
	for _, status := range []int{http.StatusOK, http.StatusAccepted, http.StatusNoContent} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			_, captured := newCaptureServer(t, func(w http.ResponseWriter, _ *http.Request, _ *capturedReq) { w.WriteHeader(status) })
			if err := DeleteWikiPage(context.Background(), "o", "r", "Guides%2FSetup"); err != nil {
				t.Fatal(err)
			}
			if captured.method != http.MethodDelete || captured.path != "/api/v1/repos/o/r/wiki/page/Guides/Setup" {
				t.Fatalf("request mismatch: %s %s", captured.method, captured.path)
			}
		})
	}
}

func TestWikiErrorsMapSentinels(t *testing.T) {
	tests := []struct {
		name   string
		status int
		want   error
		call   func() error
	}{
		{"get-404", http.StatusNotFound, ErrNotFound, func() error { _, err := GetWikiPage(context.Background(), "o", "r", "missing"); return err }},
		{"create-403", http.StatusForbidden, ErrUnauthorized, func() error { _, err := CreateWikiPage(context.Background(), "o", "r", "T", "YQ==", "m"); return err }},
		{"edit-403", http.StatusForbidden, ErrUnauthorized, func() error {
			_, err := EditWikiPage(context.Background(), "o", "r", "T", "T", "YQ==", "m")
			return err
		}},
		{"delete-403", http.StatusForbidden, ErrUnauthorized, func() error { return DeleteWikiPage(context.Background(), "o", "r", "T") }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			newCaptureServer(t, func(w http.ResponseWriter, _ *http.Request, _ *capturedReq) { w.WriteHeader(tc.status) })
			if err := tc.call(); !errors.Is(err, tc.want) {
				t.Fatalf("expected %v, got %v", tc.want, err)
			}
		})
	}
}
