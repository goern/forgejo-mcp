// SPDX-License-Identifier: GPL-3.0-or-later

package resource

import (
	"errors"
	"testing"
)

func TestParseIssues(t *testing.T) {
	p, err := ParseIssues("forgejo://repo/acme/widgets/issues")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Owner != "acme" || p.Repo != "widgets" {
		t.Fatalf("got %+v", p)
	}
}

func TestParseIssues_IgnoresQuery(t *testing.T) {
	p, err := ParseIssues("forgejo://repo/acme/widgets/issues?state=all&labels=bug&page=2&limit=5")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Owner != "acme" || p.Repo != "widgets" {
		t.Fatalf("query string must not disturb path parsing, got %+v", p)
	}
}

func TestParseIssues_RejectsSingleIssueURI(t *testing.T) {
	// forgejo://repo/{owner}/{repo}/issue/{index} is a different resource; the
	// near-miss is the realistic typo, so it must not silently resolve here.
	if _, err := ParseIssues("forgejo://repo/acme/widgets/issue/42"); !errors.Is(err, ErrInvalidParams) {
		t.Fatalf("expected ErrInvalidParams, got %v", err)
	}
}

func TestParseIssues_RejectsWrongHost(t *testing.T) {
	if _, err := ParseIssues("forgejo://org/acme/issues"); !errors.Is(err, ErrInvalidParams) {
		t.Fatalf("expected ErrInvalidParams, got %v", err)
	}
}

func TestParseIssueComments(t *testing.T) {
	p, err := ParseIssueComments("forgejo://repo/acme/widgets/issue/42/comments")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Owner != "acme" || p.Repo != "widgets" || p.Kind != "issue" || p.Index != 42 {
		t.Fatalf("got %+v", p)
	}
}

func TestParseIssueComments_PRKind(t *testing.T) {
	p, err := ParseIssueComments("forgejo://repo/acme/widgets/pr/7/comments")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Kind != "pr" || p.Index != 7 {
		t.Fatalf("got %+v", p)
	}
}

func TestParseIssueComments_RejectsBadKind(t *testing.T) {
	if _, err := ParseIssueComments("forgejo://repo/acme/widgets/wiki/7/comments"); !errors.Is(err, ErrInvalidParams) {
		t.Fatalf("expected ErrInvalidParams, got %v", err)
	}
}

func TestParseIssueComments_RejectsNonNumericIndex(t *testing.T) {
	if _, err := ParseIssueComments("forgejo://repo/acme/widgets/issue/abc/comments"); !errors.Is(err, ErrInvalidParams) {
		t.Fatalf("expected ErrInvalidParams, got %v", err)
	}
}

func TestParseIssueComments_RejectsSingleCommentURI(t *testing.T) {
	// .../comment/{id} is the existing single-comment resource.
	if _, err := ParseIssueComments("forgejo://repo/acme/widgets/issue/42/comment/9"); !errors.Is(err, ErrInvalidParams) {
		t.Fatalf("expected ErrInvalidParams, got %v", err)
	}
}
