// SPDX-License-Identifier: GPL-3.0-or-later

package actions

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forgejo"
	"github.com/mark3labs/mcp-go/mcp"
)

func setupSecretsMockServer(t *testing.T, handler http.HandlerFunc) (*httptest.Server, *string, *string) {
	t.Helper()
	var capturedPath, capturedBody string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		if r.Body != nil {
			defer r.Body.Close()
			buf := make([]byte, 4096)
			n, _ := r.Body.Read(buf)
			capturedBody = string(buf[:n])
		}
		handler(w, r)
	}))

	client, err := forgejo_sdk.NewClient(srv.URL, forgejo_sdk.SetForgejoVersion("7.0.0"))
	if err != nil {
		t.Fatalf("creating test client: %v", err)
	}
	forgejo.SetClientForTesting(client)

	return srv, &capturedPath, &capturedBody
}

func TestListRepoActionSecretsFn_OmitsValues(t *testing.T) {
	srv, capturedPath, _ := setupSecretsMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"name":"DEPLOY_TOKEN","data":"should-not-leak","created_at":"2026-01-01T00:00:00Z"}]`))
	})
	defer srv.Close()

	result, err := ListRepoActionSecretsFn(context.Background(), newCallToolRequest(map[string]interface{}{
		"owner": "o", "repo": "r",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error")
	}

	if *capturedPath != "/api/v1/repos/o/r/actions/secrets" {
		t.Fatalf("path: %s", *capturedPath)
	}

	text := result.Content[0].(mcp.TextContent).Text
	if strings.Contains(text, "should-not-leak") {
		t.Fatalf("secret value leaked into tool result: %s", text)
	}
	if !strings.Contains(text, "DEPLOY_TOKEN") {
		t.Fatalf("expected secret name in result: %s", text)
	}
}

func TestCreateOrUpdateRepoActionSecretFn_SendsPutWithData(t *testing.T) {
	srv, capturedPath, capturedBody := setupSecretsMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("expected PUT, got %s", r.Method)
		}
		w.WriteHeader(http.StatusNoContent)
	})
	defer srv.Close()

	result, err := CreateOrUpdateRepoActionSecretFn(context.Background(), newCallToolRequest(map[string]interface{}{
		"owner": "o", "repo": "r", "secret_name": "DEPLOY_TOKEN", "data": "super-secret-value",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error")
	}

	if *capturedPath != "/api/v1/repos/o/r/actions/secrets/DEPLOY_TOKEN" {
		t.Fatalf("path: %s", *capturedPath)
	}
	var body map[string]interface{}
	if err := json.Unmarshal([]byte(*capturedBody), &body); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if body["data"] != "super-secret-value" {
		t.Fatalf("expected data in request body, got: %v", body)
	}

	text := result.Content[0].(mcp.TextContent).Text
	if strings.Contains(text, "super-secret-value") {
		t.Fatalf("secret value leaked into tool result: %s", text)
	}
}

func TestCreateOrUpdateRepoActionSecretFn_MissingData(t *testing.T) {
	_, err := CreateOrUpdateRepoActionSecretFn(context.Background(), newCallToolRequest(map[string]interface{}{
		"owner": "o", "repo": "r", "secret_name": "DEPLOY_TOKEN",
	}))
	if err == nil {
		t.Fatal("expected error for missing data, got nil")
	}
}

func TestDeleteRepoActionSecretFn_SendsDelete(t *testing.T) {
	srv, capturedPath, _ := setupSecretsMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("expected DELETE, got %s", r.Method)
		}
		w.WriteHeader(http.StatusNoContent)
	})
	defer srv.Close()

	result, err := DeleteRepoActionSecretFn(context.Background(), newCallToolRequest(map[string]interface{}{
		"owner": "o", "repo": "r", "secret_name": "DEPLOY_TOKEN",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error")
	}
	if *capturedPath != "/api/v1/repos/o/r/actions/secrets/DEPLOY_TOKEN" {
		t.Fatalf("path: %s", *capturedPath)
	}
}

func TestListOrgActionSecretsFn_OmitsValues(t *testing.T) {
	srv, capturedPath, _ := setupSecretsMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"name":"NPM_TOKEN","data":"should-not-leak"}]`))
	})
	defer srv.Close()

	result, err := ListOrgActionSecretsFn(context.Background(), newCallToolRequest(map[string]interface{}{
		"org": "acme",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if *capturedPath != "/api/v1/orgs/acme/actions/secrets" {
		t.Fatalf("path: %s", *capturedPath)
	}
	text := result.Content[0].(mcp.TextContent).Text
	if strings.Contains(text, "should-not-leak") {
		t.Fatalf("secret value leaked into tool result: %s", text)
	}
}

func TestCreateOrUpdateOrgActionSecretFn_MissingOrg(t *testing.T) {
	_, err := CreateOrUpdateOrgActionSecretFn(context.Background(), newCallToolRequest(map[string]interface{}{
		"secret_name": "NPM_TOKEN", "data": "value",
	}))
	if err == nil {
		t.Fatal("expected error for missing org, got nil")
	}
}

func TestDeleteOrgActionSecretFn_SendsDelete(t *testing.T) {
	srv, capturedPath, _ := setupSecretsMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("expected DELETE, got %s", r.Method)
		}
		w.WriteHeader(http.StatusNoContent)
	})
	defer srv.Close()

	result, err := DeleteOrgActionSecretFn(context.Background(), newCallToolRequest(map[string]interface{}{
		"org": "acme", "secret_name": "NPM_TOKEN",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error")
	}
	if *capturedPath != "/api/v1/orgs/acme/actions/secrets/NPM_TOKEN" {
		t.Fatalf("path: %s", *capturedPath)
	}
}
