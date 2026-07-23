// SPDX-License-Identifier: GPL-3.0-or-later

package resource

import (
	"errors"
	"net/url"
	"strings"
	"testing"
)

func TestParseWiki(t *testing.T) {
	const uri = "forgejo://repo/acme/docs/wiki/Guides%2FSetup"
	u, err := url.Parse(uri)
	if err != nil {
		t.Fatal(err)
	}
	if u.RawPath == "" || !strings.Contains(u.EscapedPath(), "%2F") {
		t.Fatalf("encoded slash was not retained: RawPath=%q EscapedPath=%q", u.RawPath, u.EscapedPath())
	}
	got, err := ParseWiki(uri)
	if err != nil {
		t.Fatal(err)
	}
	if got.Owner != "acme" || got.Repo != "docs" || got.PageName != "Guides/Setup" {
		t.Fatalf("unexpected: %+v", got)
	}
}

func TestParseWikiRejectsEmptyPageName(t *testing.T) {
	_, err := ParseWiki("forgejo://repo/acme/docs/wiki/")
	if !errors.Is(err, ErrInvalidParams) {
		t.Fatalf("expected invalid params, got %v", err)
	}
}

func TestParseWikiRejectsWhitespacePageName(t *testing.T) {
	_, err := ParseWiki("forgejo://repo/acme/docs/wiki/%20")
	if !errors.Is(err, ErrInvalidParams) {
		t.Fatalf("expected invalid params, got %v", err)
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
	if !strings.Contains(err.Error(), "%2F") {
		t.Fatalf("expected encoded-slash guidance, got %v", err)
	}
}
