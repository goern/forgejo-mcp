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
