// SPDX-License-Identifier: GPL-3.0-or-later

package packages

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

func TestLinkPackageFn_MissingArgsSendNoRequest(t *testing.T) {
	_, capture := setupPackageAPIServer(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("no request should be sent")
	})

	if _, err := LinkPackageFn(context.Background(), newCallToolRequest(map[string]interface{}{
		"type": "generic", "name": "dist", "repo": "core",
	})); err == nil {
		t.Fatal("expected error for missing owner")
	}
	if _, err := LinkPackageFn(context.Background(), newCallToolRequest(map[string]interface{}{
		"owner": "o", "type": "", "name": "dist", "repo": "core",
	})); err == nil {
		t.Fatal("expected error for empty type")
	}
	if _, err := LinkPackageFn(context.Background(), newCallToolRequest(map[string]interface{}{
		"owner": "o", "type": "generic", "name": "", "repo": "core",
	})); err == nil {
		t.Fatal("expected error for empty name")
	}
	if _, err := LinkPackageFn(context.Background(), newCallToolRequest(map[string]interface{}{
		"owner": "o", "type": "generic", "name": "dist",
	})); err == nil {
		t.Fatal("expected error for missing repo")
	}
	if capture.count != 0 {
		t.Errorf("request sent: count=%d", capture.count)
	}
}

func TestUnlinkPackageFn_MissingArgsSendNoRequest(t *testing.T) {
	_, capture := setupPackageAPIServer(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("no request should be sent")
	})

	if _, err := UnlinkPackageFn(context.Background(), newCallToolRequest(map[string]interface{}{
		"type": "generic", "name": "dist",
	})); err == nil {
		t.Fatal("expected error for missing owner")
	}
	if _, err := UnlinkPackageFn(context.Background(), newCallToolRequest(map[string]interface{}{
		"owner": "o", "type": "", "name": "dist",
	})); err == nil {
		t.Fatal("expected error for empty type")
	}
	if _, err := UnlinkPackageFn(context.Background(), newCallToolRequest(map[string]interface{}{
		"owner": "o", "type": "generic",
	})); err == nil {
		t.Fatal("expected error for missing name")
	}
	if capture.count != 0 {
		t.Errorf("request sent: count=%d", capture.count)
	}
}

func TestLinkPackageFn_RepoSlashSendsNoRequest(t *testing.T) {
	_, capture := setupPackageAPIServer(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("no request should be sent")
	})

	_, err := LinkPackageFn(context.Background(), newCallToolRequest(map[string]interface{}{
		"owner": "cnvr", "type": "npm", "name": "oss", "repo": "cnvr/oss",
	}))
	if err == nil {
		t.Fatal("repo containing / must be an error")
	}
	if !strings.Contains(err.Error(), "not owner/repo") {
		t.Errorf("error should name the rule: %v", err)
	}
	if capture.count != 0 {
		t.Errorf("request sent: count=%d", capture.count)
	}
}

func TestLinkPackageFn_EscapesSlashInName(t *testing.T) {
	_, capture := setupPackageAPIServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{not json`))
	})

	result, err := LinkPackageFn(context.Background(), newCallToolRequest(map[string]interface{}{
		"owner": "o", "type": "container", "name": "youscore/core", "repo": "core",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capture.method != http.MethodPost {
		t.Errorf("method: %s", capture.method)
	}
	want := "/api/v1/packages/o/container/youscore%2Fcore/-/link/core"
	if capture.escapedPath != want {
		t.Errorf("path retargeted: path=%s escaped=%s want=%s", capture.path, capture.escapedPath, want)
	}
	decoded := decodePackageResult[linkPackageResult](t, result)
	if decoded.Status != "linked" || decoded.Repo != "core" || decoded.Name != "youscore/core" {
		t.Errorf("result: %+v", decoded)
	}
}

func TestLinkPackageFn_OwnerSlashDoesNotRetarget(t *testing.T) {
	_, capture := setupPackageAPIServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})

	_, err := LinkPackageFn(context.Background(), newCallToolRequest(map[string]interface{}{
		"owner": "acme/org", "type": "generic", "name": "dist", "repo": "core",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "/api/v1/packages/acme%2Forg/generic/dist/-/link/core"
	if capture.escapedPath != want {
		t.Errorf("path retargeted: path=%s escaped=%s want=%s", capture.path, capture.escapedPath, want)
	}
}

func TestLinkPackageFn_CreatedEmptyBody(t *testing.T) {
	_, capture := setupPackageAPIServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})

	result, err := LinkPackageFn(context.Background(), newCallToolRequest(map[string]interface{}{
		"owner": "o", "type": "generic", "name": "dist", "repo": "core",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capture.method != http.MethodPost {
		t.Errorf("method: %s", capture.method)
	}
	if capture.path != "/api/v1/packages/o/generic/dist/-/link/core" {
		t.Errorf("path: %s", capture.path)
	}
	decoded := decodePackageResult[linkPackageResult](t, result)
	if decoded.Status != "linked" || decoded.Owner != "o" || decoded.Type != "generic" || decoded.Name != "dist" || decoded.Repo != "core" {
		t.Errorf("result: %+v", decoded)
	}
	text := resultText(t, result)
	if strings.Contains(text, `"already_linked"`) {
		t.Errorf("must not invent already_linked: %s", text)
	}
}

func TestUnlinkPackageFn_NoContent(t *testing.T) {
	_, capture := setupPackageAPIServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
		_, _ = w.Write([]byte(`{not json`))
	})

	result, err := UnlinkPackageFn(context.Background(), newCallToolRequest(map[string]interface{}{
		"owner": "o", "type": "generic", "name": "dist",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capture.method != http.MethodPost {
		t.Errorf("method: %s", capture.method)
	}
	if capture.path != "/api/v1/packages/o/generic/dist/-/unlink" {
		t.Errorf("path: %s", capture.path)
	}
	decoded := decodePackageResult[unlinkPackageResult](t, result)
	if decoded.Status != "unlinked" || decoded.Owner != "o" || decoded.Name != "dist" {
		t.Errorf("result: %+v", decoded)
	}
}

func TestLinkPackageFn_NotFoundPackageIsError(t *testing.T) {
	_, capture := setupPackageAPIServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"package does not exist"}`))
	})

	result, err := LinkPackageFn(context.Background(), newCallToolRequest(map[string]interface{}{
		"owner": "o", "type": "generic", "name": "missing", "repo": "core",
	}))
	if err == nil {
		t.Fatal("404 package must be an MCP error")
	}
	if result != nil {
		t.Errorf("expected nil result, got %+v", result)
	}
	if capture.method != http.MethodPost {
		t.Errorf("POST must still be sent, got %s", capture.method)
	}
	if strings.Contains(err.Error(), `"linked"`) {
		t.Errorf("must not map 404 to linked: %v", err)
	}
}

func TestLinkPackageFn_NotFoundRepoIsError(t *testing.T) {
	_, capture := setupPackageAPIServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"repository does not exist"}`))
	})

	result, err := LinkPackageFn(context.Background(), newCallToolRequest(map[string]interface{}{
		"owner": "o", "type": "generic", "name": "dist", "repo": "missing",
	}))
	if err == nil {
		t.Fatal("404 repo must be an MCP error")
	}
	if result != nil {
		t.Errorf("expected nil result, got %+v", result)
	}
	if capture.count != 1 {
		t.Errorf("expected one POST, count=%d", capture.count)
	}
}

func TestLinkPackageFn_ForbiddenIsError(t *testing.T) {
	setupPackageAPIServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"message":"forbidden"}`))
	})

	result, err := LinkPackageFn(context.Background(), newCallToolRequest(map[string]interface{}{
		"owner": "o", "type": "generic", "name": "dist", "repo": "core",
	}))
	if err == nil {
		t.Fatal("403 must be an MCP error")
	}
	if result != nil {
		t.Errorf("expected nil result, got %+v", result)
	}
}

func TestLinkPackageFn_AlreadyLinkedIsError(t *testing.T) {
	_, capture := setupPackageAPIServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"message":"package is already linked"}`))
	})

	result, err := LinkPackageFn(context.Background(), newCallToolRequest(map[string]interface{}{
		"owner": "o", "type": "generic", "name": "dist", "repo": "core",
	}))
	if err == nil {
		t.Fatal("400 already-linked must be an MCP error")
	}
	if result != nil {
		t.Errorf("expected nil result, got %+v", result)
	}
	if strings.Contains(err.Error(), "already_linked") {
		t.Errorf("must not invent already_linked: %v", err)
	}
	if capture.method != http.MethodPost {
		t.Errorf("POST must still be sent, got %s", capture.method)
	}
}

func TestUnlinkPackageFn_NotFoundIsError(t *testing.T) {
	setupPackageAPIServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"package does not exist"}`))
	})

	result, err := UnlinkPackageFn(context.Background(), newCallToolRequest(map[string]interface{}{
		"owner": "o", "type": "generic", "name": "missing",
	}))
	if err == nil {
		t.Fatal("404 unlink must be an MCP error")
	}
	if result != nil {
		t.Errorf("expected nil result, got %+v", result)
	}
}

func TestUnlinkPackageFn_NotLinkedIsError(t *testing.T) {
	setupPackageAPIServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"message":"package is not linked"}`))
	})

	result, err := UnlinkPackageFn(context.Background(), newCallToolRequest(map[string]interface{}{
		"owner": "o", "type": "generic", "name": "dist",
	}))
	if err == nil {
		t.Fatal("400 not-linked must be an MCP error")
	}
	if result != nil {
		t.Errorf("expected nil result, got %+v", result)
	}
}

func TestLinkPackageFn_ThenGetPackageShowsRepository(t *testing.T) {
	_, capture := setupPackageAPIServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/link/") {
			w.WriteHeader(http.StatusCreated)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(fatPackageJSON))
	})

	linkResult, err := LinkPackageFn(context.Background(), newCallToolRequest(map[string]interface{}{
		"owner": "alice", "type": "container", "name": "core", "repo": "core",
	}))
	if err != nil {
		t.Fatalf("link: %v", err)
	}
	decodedLink := decodePackageResult[linkPackageResult](t, linkResult)
	if decodedLink.Status != "linked" {
		t.Errorf("link result: %+v", decodedLink)
	}

	getResult, err := GetPackageFn(context.Background(), newCallToolRequest(map[string]interface{}{
		"owner": "alice", "type": "container", "name": "core", "version": "1.0.0",
	}))
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	decodedGet := decodePackageResult[packageVersion](t, getResult)
	if decodedGet.Repository != "alice/core" {
		t.Errorf("repository: %+v", decodedGet)
	}
	if capture.count != 2 {
		t.Errorf("expected link POST then get GET, count=%d", capture.count)
	}
}
