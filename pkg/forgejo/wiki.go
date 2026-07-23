// SPDX-License-Identifier: GPL-3.0-or-later

package forgejo

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
)

type WikiPerson struct {
	Name  string `json:"name"`
	Email string `json:"email,omitempty"`
	Date  string `json:"date,omitempty"`
}

type WikiCommit struct {
	SHA     string     `json:"sha"`
	Author  WikiPerson `json:"author"`
	Message string     `json:"message"`
}

type WikiPageMeta struct {
	Title      string     `json:"title"`
	HTMLURL    string     `json:"html_url,omitempty"`
	SubURL     string     `json:"sub_url"`
	LastCommit WikiCommit `json:"last_commit"`
}

type WikiPage struct {
	WikiPageMeta
	ContentBase64 string `json:"content_base64"`
	CommitCount   int    `json:"commit_count"`
	Sidebar       string `json:"sidebar,omitempty"`
	Footer        string `json:"footer,omitempty"`
}

type WikiRevisions struct {
	Commits []WikiCommit `json:"commits"`
	Count   int          `json:"count"`
}

type wikiWriteOptions struct {
	Title         string `json:"title"`
	ContentBase64 string `json:"content_base64"`
	Message       string `json:"message"`
}

func wikiRepoPath(owner, repo string) string {
	return fmt.Sprintf("/repos/%s/%s/wiki", url.PathEscape(owner), url.PathEscape(repo))
}

func wikiPageSegment(pageName string) string {
	decoded, err := url.PathUnescape(pageName)
	if err != nil {
		decoded = pageName
	}
	return url.PathEscape(decoded)
}

func ListWikiPages(ctx context.Context, owner, repo string, page, limit int) ([]WikiPageMeta, error) {
	path := fmt.Sprintf("%s/pages?page=%d&limit=%d", wikiRepoPath(owner, repo), page, limit)
	var pages []WikiPageMeta
	if err := DoJSONList(ctx, http.MethodGet, path, &pages); err != nil {
		return nil, err
	}
	return pages, nil
}

func GetWikiPage(ctx context.Context, owner, repo, pageName string) (*WikiPage, error) {
	var page WikiPage
	path := fmt.Sprintf("%s/page/%s", wikiRepoPath(owner, repo), wikiPageSegment(pageName))
	if err := DoJSON(ctx, http.MethodGet, path, nil, &page); err != nil {
		return nil, err
	}
	return &page, nil
}

func GetWikiPageRevisions(ctx context.Context, owner, repo, pageName string, page, limit int) (*WikiRevisions, error) {
	var revisions WikiRevisions
	path := fmt.Sprintf("%s/revisions/%s?page=%d&limit=%d", wikiRepoPath(owner, repo), wikiPageSegment(pageName), page, limit)
	if err := DoJSON(ctx, http.MethodGet, path, nil, &revisions); err != nil {
		return nil, err
	}
	return &revisions, nil
}

func CreateWikiPage(ctx context.Context, owner, repo, title, contentBase64, message string) (*WikiPage, error) {
	var page WikiPage
	body := wikiWriteOptions{Title: title, ContentBase64: contentBase64, Message: message}
	if err := DoJSON(ctx, http.MethodPost, wikiRepoPath(owner, repo)+"/new", body, &page); err != nil {
		return nil, err
	}
	return &page, nil
}

func EditWikiPage(ctx context.Context, owner, repo, pageName, title, contentBase64, message string) (*WikiPage, error) {
	var page WikiPage
	body := wikiWriteOptions{Title: title, ContentBase64: contentBase64, Message: message}
	path := fmt.Sprintf("%s/page/%s", wikiRepoPath(owner, repo), wikiPageSegment(pageName))
	if err := DoJSON(ctx, http.MethodPatch, path, body, &page); err != nil {
		return nil, err
	}
	return &page, nil
}

func DeleteWikiPage(ctx context.Context, owner, repo, pageName string) error {
	path := fmt.Sprintf("%s/page/%s", wikiRepoPath(owner, repo), wikiPageSegment(pageName))
	return DoJSON(ctx, http.MethodDelete, path, nil, nil)
}
