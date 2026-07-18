// SPDX-License-Identifier: GPL-3.0-or-later

package resource

import (
	"errors"
	"testing"
)

func TestParseWiki(t *testing.T) {
	got, err := ParseWiki("forgejo://repo/acme/docs/wiki/Guides%2FSetup")
	if err != nil {
		t.Fatal(err)
	}
	if got.Owner != "acme" || got.Repo != "docs" || got.PageName != "Guides/Setup" {
		t.Fatalf("unexpected: %+v", got)
	}
}

func TestParseWikiSpace(t *testing.T) {
	got, err := ParseWiki("forgejo://repo/acme/docs/wiki/Getting%20Started")
	if err != nil || got.PageName != "Getting Started" {
		t.Fatalf("got=%+v err=%v", got, err)
	}
}

func TestParseWikiLiteralSlashIsGuidedError(t *testing.T) {
	_, err := ParseWiki("forgejo://repo/acme/docs/wiki/Guides/Setup")
	if !errors.Is(err, ErrInvalidParams) {
		t.Fatalf("expected invalid params, got %v", err)
	}
}
