// SPDX-License-Identifier: GPL-3.0-or-later

package actions

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

func TestCancelWorkflowRunFn_RunningRun(t *testing.T) {
	_, capture := setupActionAPIServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	result, err := CancelWorkflowRunFn(context.Background(), newCallToolRequest(map[string]interface{}{
		"owner": "o", "repo": "r", "run_id": float64(42),
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	decoded := decodeActionResult[workflowRunControlResult](t, result)
	if capture.method != http.MethodPost {
		t.Fatalf("method: %s", capture.method)
	}
	if capture.path != "/api/v1/repos/o/r/actions/runs/42/cancel" {
		t.Fatalf("path: %s", capture.path)
	}
	if decoded.RunID != 42 || decoded.Status != "cancelled" {
		t.Fatalf("result: %+v", decoded)
	}
}

func TestCancelWorkflowRunFn_AlreadyFinishedIsSuccess(t *testing.T) {
	setupActionAPIServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	result, err := CancelWorkflowRunFn(context.Background(), newCallToolRequest(map[string]interface{}{
		"owner": "o", "repo": "r", "run_id": float64(7),
	}))
	if err != nil {
		t.Fatalf("already-finished 204 must be success, got %v", err)
	}
	decoded := decodeActionResult[workflowRunControlResult](t, result)
	if decoded.Status != "cancelled" {
		t.Fatalf("result: %+v", decoded)
	}
}

func TestDeleteWorkflowRunFn_CompletedRun(t *testing.T) {
	_, capture := setupActionAPIServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	result, err := DeleteWorkflowRunFn(context.Background(), newCallToolRequest(map[string]interface{}{
		"owner": "o", "repo": "r", "run_id": float64(42),
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	decoded := decodeActionResult[workflowRunControlResult](t, result)
	if capture.method != http.MethodDelete {
		t.Fatalf("method: %s", capture.method)
	}
	if capture.path != "/api/v1/repos/o/r/actions/runs/42" {
		t.Fatalf("path: %s", capture.path)
	}
	if decoded.RunID != 42 || decoded.Status != "deleted" {
		t.Fatalf("result: %+v", decoded)
	}
}

func TestDeleteWorkflowRunFn_LiveRunIsError(t *testing.T) {
	for _, status := range []int{http.StatusBadRequest, http.StatusInternalServerError} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			_, capture := setupActionAPIServer(t, func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(status)
				_, _ = w.Write([]byte(`{"message":"cannot delete run 42 because it has not completed yet"}`))
			})

			result, err := DeleteWorkflowRunFn(context.Background(), newCallToolRequest(map[string]interface{}{
				"owner": "o", "repo": "r", "run_id": float64(42),
			}))
			if err == nil {
				t.Fatal("live-run delete must be an error")
			}
			if result != nil {
				t.Fatalf("expected nil result, got %+v", result)
			}
			if strings.Contains(err.Error(), `"status":"deleted"`) {
				t.Fatalf("error mapped to success: %v", err)
			}
			if capture.method != http.MethodDelete || capture.count != 1 {
				t.Fatalf("DELETE still has to be sent: method=%s count=%d", capture.method, capture.count)
			}
		})
	}
}

func TestCancelWorkflowRunFn_MissingArgsSendNoRequest(t *testing.T) {
	_, capture := setupActionAPIServer(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("no request should be sent")
	})

	for _, args := range []map[string]interface{}{
		{"repo": "r", "run_id": float64(1)},
		{"owner": "o", "run_id": float64(1)},
		{"owner": "o", "repo": "r"},
		{"owner": "o", "repo": "r", "run_id": float64(0)},
	} {
		if _, err := CancelWorkflowRunFn(context.Background(), newCallToolRequest(args)); err == nil {
			t.Fatalf("expected error for %+v", args)
		}
		if capture.count != 0 {
			t.Fatalf("request sent for %+v", args)
		}
	}
}

func TestCancelWorkflowRunFn_OwnerSlashDoesNotRetarget(t *testing.T) {
	_, capture := setupActionAPIServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	_, err := CancelWorkflowRunFn(context.Background(), newCallToolRequest(map[string]interface{}{
		"owner": "o/x", "repo": "r", "run_id": float64(42),
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capture.escapedPath != "/api/v1/repos/o%2Fx/r/actions/runs/42/cancel" {
		t.Fatalf("path retargeted: path=%s escaped=%s", capture.path, capture.escapedPath)
	}
}
