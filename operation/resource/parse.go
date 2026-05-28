package resource

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// ErrInvalidParams indicates a URI parse failure that should map to JSON-RPC -32602.
var ErrInvalidParams = errors.New("invalid params")

// OwnerParams holds parsed fields from forgejo://owner/{owner}.
type OwnerParams struct {
	Owner string
}

// RepoParams holds parsed fields from forgejo://repo/{owner}/{repo}.
type RepoParams struct {
	Owner string
	Repo  string
}

// CommitParams holds parsed fields from forgejo://repo/{owner}/{repo}/commit/{sha}.
// SHA must be exactly 40 hex characters.
type CommitParams struct {
	Owner string
	Repo  string
	SHA   string
}

// IssueParams holds parsed fields from forgejo://repo/{owner}/{repo}/issue/{index}.
type IssueParams struct {
	Owner string
	Repo  string
	Index int64
}

// PRParams holds parsed fields from forgejo://repo/{owner}/{repo}/pr/{index}.
type PRParams struct {
	Owner string
	Repo  string
	Index int64
}

// CommentParams holds parsed fields from forgejo://repo/{owner}/{repo}/{kind}/{index}/comment/{id}.
// Kind is constrained to "issue" or "pr".
type CommentParams struct {
	Owner string
	Repo  string
	Kind  string
	Index int64
	ID    int64
}

// StatusParams holds parsed fields from forgejo://repo/{owner}/{repo}/commit/{sha}/status.
type StatusParams struct {
	Owner string
	Repo  string
	SHA   string
}

// ParseOwner parses forgejo://owner/{owner}.
func ParseOwner(uri string) (OwnerParams, error) {
	u, err := parseForgejoURI(uri)
	if err != nil {
		return OwnerParams{}, err
	}
	// host = "owner", path = "/{owner}"
	if u.Host != "owner" {
		return OwnerParams{}, fmt.Errorf("%w: expected forgejo://owner/{owner}, got %q", ErrInvalidParams, uri)
	}
	parts := splitPath(u.Path)
	if len(parts) != 1 || parts[0] == "" {
		return OwnerParams{}, fmt.Errorf("%w: expected forgejo://owner/{owner}, got %q", ErrInvalidParams, uri)
	}
	return OwnerParams{Owner: parts[0]}, nil
}

// ParseRepo parses forgejo://repo/{owner}/{repo}.
func ParseRepo(uri string) (RepoParams, error) {
	u, err := parseForgejoURI(uri)
	if err != nil {
		return RepoParams{}, err
	}
	if u.Host != "repo" {
		return RepoParams{}, fmt.Errorf("%w: expected forgejo://repo/..., got %q", ErrInvalidParams, uri)
	}
	parts := splitPath(u.Path)
	if len(parts) != 2 {
		return RepoParams{}, fmt.Errorf("%w: expected forgejo://repo/{owner}/{repo}, got %q", ErrInvalidParams, uri)
	}
	return RepoParams{Owner: parts[0], Repo: parts[1]}, nil
}

// ParseCommit parses forgejo://repo/{owner}/{repo}/commit/{sha}.
// Returns an error if sha is not exactly 40 lowercase hex characters.
func ParseCommit(uri string) (CommitParams, error) {
	u, err := parseForgejoURI(uri)
	if err != nil {
		return CommitParams{}, err
	}
	if u.Host != "repo" {
		return CommitParams{}, fmt.Errorf("%w: expected forgejo://repo/..., got %q", ErrInvalidParams, uri)
	}
	parts := splitPath(u.Path)
	// parts: [owner, repo, "commit", sha]
	if len(parts) != 4 || parts[2] != "commit" {
		return CommitParams{}, fmt.Errorf("%w: expected forgejo://repo/{owner}/{repo}/commit/{sha}, got %q", ErrInvalidParams, uri)
	}
	sha := parts[3]
	if err := validateSHA(sha); err != nil {
		return CommitParams{}, fmt.Errorf("%w: invalid URI %q: %s", ErrInvalidParams, uri, err)
	}
	return CommitParams{Owner: parts[0], Repo: parts[1], SHA: sha}, nil
}

// ParseIssue parses forgejo://repo/{owner}/{repo}/issue/{index}.
func ParseIssue(uri string) (IssueParams, error) {
	u, err := parseForgejoURI(uri)
	if err != nil {
		return IssueParams{}, err
	}
	if u.Host != "repo" {
		return IssueParams{}, fmt.Errorf("%w: expected forgejo://repo/..., got %q", ErrInvalidParams, uri)
	}
	parts := splitPath(u.Path)
	// parts: [owner, repo, "issue", index]
	if len(parts) != 4 || parts[2] != "issue" {
		return IssueParams{}, fmt.Errorf("%w: expected forgejo://repo/{owner}/{repo}/issue/{index}, got %q", ErrInvalidParams, uri)
	}
	index, err := strconv.ParseInt(parts[3], 10, 64)
	if err != nil {
		return IssueParams{}, fmt.Errorf("%w: invalid URI %q: index must be numeric", ErrInvalidParams, uri)
	}
	return IssueParams{Owner: parts[0], Repo: parts[1], Index: index}, nil
}

// ParsePR parses forgejo://repo/{owner}/{repo}/pr/{index}.
func ParsePR(uri string) (PRParams, error) {
	u, err := parseForgejoURI(uri)
	if err != nil {
		return PRParams{}, err
	}
	if u.Host != "repo" {
		return PRParams{}, fmt.Errorf("%w: expected forgejo://repo/..., got %q", ErrInvalidParams, uri)
	}
	parts := splitPath(u.Path)
	// parts: [owner, repo, "pr", index]
	if len(parts) != 4 || parts[2] != "pr" {
		return PRParams{}, fmt.Errorf("%w: expected forgejo://repo/{owner}/{repo}/pr/{index}, got %q", ErrInvalidParams, uri)
	}
	index, err := strconv.ParseInt(parts[3], 10, 64)
	if err != nil {
		return PRParams{}, fmt.Errorf("%w: invalid URI %q: index must be numeric", ErrInvalidParams, uri)
	}
	return PRParams{Owner: parts[0], Repo: parts[1], Index: index}, nil
}

// ParseComment parses forgejo://repo/{owner}/{repo}/{kind}/{index}/comment/{id}.
// Kind must be "issue" or "pr".
func ParseComment(uri string) (CommentParams, error) {
	u, err := parseForgejoURI(uri)
	if err != nil {
		return CommentParams{}, err
	}
	if u.Host != "repo" {
		return CommentParams{}, fmt.Errorf("%w: expected forgejo://repo/..., got %q", ErrInvalidParams, uri)
	}
	parts := splitPath(u.Path)
	// parts: [owner, repo, kind, index, "comment", id]
	if len(parts) != 6 || parts[4] != "comment" {
		return CommentParams{}, fmt.Errorf("%w: expected forgejo://repo/{owner}/{repo}/{kind}/{index}/comment/{id}, got %q", ErrInvalidParams, uri)
	}
	kind := parts[2]
	if kind != "issue" && kind != "pr" {
		return CommentParams{}, fmt.Errorf("%w: invalid URI %q: kind must be 'issue' or 'pr', got %q", ErrInvalidParams, uri, kind)
	}
	index, err := strconv.ParseInt(parts[3], 10, 64)
	if err != nil {
		return CommentParams{}, fmt.Errorf("%w: invalid URI %q: index must be numeric", ErrInvalidParams, uri)
	}
	id, err := strconv.ParseInt(parts[5], 10, 64)
	if err != nil {
		return CommentParams{}, fmt.Errorf("%w: invalid URI %q: id must be numeric", ErrInvalidParams, uri)
	}
	return CommentParams{Owner: parts[0], Repo: parts[1], Kind: kind, Index: index, ID: id}, nil
}

// ParseStatus parses forgejo://repo/{owner}/{repo}/commit/{sha}/status.
func ParseStatus(uri string) (StatusParams, error) {
	u, err := parseForgejoURI(uri)
	if err != nil {
		return StatusParams{}, err
	}
	if u.Host != "repo" {
		return StatusParams{}, fmt.Errorf("%w: expected forgejo://repo/..., got %q", ErrInvalidParams, uri)
	}
	parts := splitPath(u.Path)
	// parts: [owner, repo, "commit", sha, "status"]
	if len(parts) != 5 || parts[2] != "commit" || parts[4] != "status" {
		return StatusParams{}, fmt.Errorf("%w: expected forgejo://repo/{owner}/{repo}/commit/{sha}/status, got %q", ErrInvalidParams, uri)
	}
	sha := parts[3]
	if err := validateSHA(sha); err != nil {
		return StatusParams{}, fmt.Errorf("%w: invalid URI %q: %s", ErrInvalidParams, uri, err)
	}
	return StatusParams{Owner: parts[0], Repo: parts[1], SHA: sha}, nil
}

func parseForgejoURI(uri string) (*url.URL, error) {
	u, err := url.Parse(uri)
	if err != nil {
		return nil, fmt.Errorf("%w: malformed URI %q: %s", ErrInvalidParams, uri, err)
	}
	if u.Scheme != "forgejo" {
		return nil, fmt.Errorf("%w: invalid URI scheme: expected 'forgejo', got %q", ErrInvalidParams, u.Scheme)
	}
	// Reject empty or whitespace-only path segments so that
	// forgejo://repo/foo//bar and forgejo://repo/foo/bar/ do not silently
	// alias forgejo://repo/foo/bar.  Distinct URIs must mean distinct
	// resources for content-addressable caching to be correct.
	path := strings.TrimPrefix(u.Path, "/")
	for _, seg := range strings.Split(path, "/") {
		if strings.TrimSpace(seg) == "" {
			return nil, fmt.Errorf("%w: invalid URI %q: empty or whitespace-only path segment", ErrInvalidParams, uri)
		}
	}
	return u, nil
}

// splitPath splits a URI path into non-empty segments.
func splitPath(path string) []string {
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

// validateSHA returns an error if sha is not exactly 40 lowercase hex characters.
func validateSHA(sha string) error {
	if len(sha) != 40 {
		return fmt.Errorf("%w: sha must be exactly 40 hex characters, got %d", ErrInvalidParams, len(sha))
	}
	for _, c := range sha {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return fmt.Errorf("%w: sha contains invalid character %q", ErrInvalidParams, c)
		}
	}
	return nil
}
