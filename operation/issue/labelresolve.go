// SPDX-License-Identifier: GPL-3.0-or-later

package issue

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forgejo"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

const (
	// labelCatalogPageSize is the page size used while enumerating labels for
	// name resolution. It is independent of the caller-facing page/limit on
	// list_repo_labels: that tool browses, this enumeration must be complete,
	// or a label on page two would resolve as "unknown".
	labelCatalogPageSize = 100

	// maxLabelCatalogPages caps the enumeration per scope. A server that
	// ignores paging would otherwise spin forever; the cap turns that into a
	// message, and numeric ids still work without a catalogue.
	maxLabelCatalogPages = 20
)

// labelsArgDescription keeps the four label arguments telling the same story:
// what they accept, and that a name they cannot resolve stops the call instead
// of quietly applying a subset.
func labelsArgDescription(lead string) string {
	return lead + " (comma-separated). Accepts label names or numeric IDs. A name is " +
		"matched before an ID, so a label named \"123\" wins over the ID 123. An " +
		"unrecognised name is an error and nothing is applied; list_repo_labels shows " +
		"what exists."
}

// listAssignableLabels returns every label that can be applied to an issue in
// the repository: the repository's own labels plus, for an org-owned
// repository, the organization's. Each scope is paged independently.
//
// A 404 on the org endpoint means the owner is a user, and yields no org
// labels; 401/403 surface as an error rather than silently narrowing the
// catalogue, since resolution failures here are reported to the caller as
// "unknown label".
func listAssignableLabels(ctx context.Context, owner, repo string) ([]ScopedLabel, error) {
	client, err := forgejo.Client(ctx)
	if err != nil {
		return nil, err
	}

	catalog := []ScopedLabel{}

	for page := 1; ; page++ {
		if page > maxLabelCatalogPages {
			return nil, fmt.Errorf(
				"%s/%s has more than %d labels; pass numeric label IDs instead of names",
				owner, repo, maxLabelCatalogPages*labelCatalogPageSize,
			)
		}
		repoLabels, _, lerr := client.ListRepoLabels(owner, repo, forgejo_sdk.ListLabelsOptions{
			ListOptions: forgejo_sdk.ListOptions{Page: page, PageSize: labelCatalogPageSize},
		})
		if lerr != nil {
			return nil, fmt.Errorf("list repo labels err: %w", lerr)
		}
		for _, l := range repoLabels {
			catalog = append(catalog, ScopedLabel{Label: l, Scope: "repo"})
		}
		if len(repoLabels) < labelCatalogPageSize {
			break
		}
	}

	for page := 1; ; page++ {
		if page > maxLabelCatalogPages {
			return nil, fmt.Errorf(
				"organization %s has more than %d labels; pass numeric label IDs instead of names",
				owner, maxLabelCatalogPages*labelCatalogPageSize,
			)
		}
		orgLabels, _, oerr := fetchOrgLabels(ctx, owner, page, labelCatalogPageSize)
		if oerr != nil {
			return nil, fmt.Errorf("list org labels err: %w", oerr)
		}
		catalog = append(catalog, orgLabels...)
		if len(orgLabels) < labelCatalogPageSize {
			break
		}
	}

	return catalog, nil
}

// resolveIssueLabelIDs turns a caller's comma-separated labels into the label
// IDs every Forgejo issue-label endpoint takes.
//
// Forgejo does accept names on the assignment endpoints, but it drops the ones
// it does not recognise and still answers 200 (GetLabelIDsInRepoByNames
// "silently ignores label names that do not belong to the repository"). Half an
// applied request reported as success is worse for an agent than a failure, so
// names are resolved here and an unresolved token aborts the call before any
// write.
//
// A token is matched as a name first and only then as an ID, so a label
// literally named "123" stays reachable through an argument documented to take
// names. An ID is honoured only when the catalogue has it, which turns a stale
// ID into the same clear error as a stale name.
func resolveIssueLabelIDs(ctx context.Context, owner, repo, csv string) ([]int64, error) {
	tokens := splitCSV(csv)
	if len(tokens) == 0 {
		return nil, fmt.Errorf("labels names nothing: pass at least one label name or numeric ID")
	}

	catalog, err := listAssignableLabels(ctx, owner, repo)
	if err != nil {
		return nil, err
	}

	byName := make(map[string][]ScopedLabel, len(catalog))
	byID := make(map[int64]ScopedLabel, len(catalog))
	for _, l := range catalog {
		byName[l.Name] = append(byName[l.Name], l)
		byID[l.ID] = l
	}

	ids := make([]int64, 0, len(tokens))
	seen := make(map[int64]bool, len(tokens))
	for _, token := range tokens {
		id, rerr := resolveOneLabel(token, byName, byID)
		if rerr != nil {
			return nil, rerr
		}
		if seen[id] {
			continue
		}
		seen[id] = true
		ids = append(ids, id)
	}
	return ids, nil
}

func resolveOneLabel(token string, byName map[string][]ScopedLabel, byID map[int64]ScopedLabel) (int64, error) {
	switch matches := byName[token]; len(matches) {
	case 1:
		return matches[0].ID, nil
	case 0:
		// Fall through to the numeric form below.
	default:
		return 0, fmt.Errorf(
			"label %q is ambiguous: it matches %s; pass the numeric label ID instead",
			token, describeLabelScopes(matches),
		)
	}

	if id, err := strconv.ParseInt(token, 10, 64); err == nil {
		if _, ok := byID[id]; ok {
			return id, nil
		}
		return 0, fmt.Errorf(
			"label %q is neither a label name nor the ID of a label on this issue's repository or organization; list_repo_labels shows what exists",
			token,
		)
	}

	return 0, fmt.Errorf(
		"unknown label %q: no repository or organization label has that name; list_repo_labels shows what exists, create_repo_label adds one",
		token,
	)
}

// describeLabelScopes renders the colliding labels as "repository label 7 and
// organization label 12", so the caller can pick an ID without a second call.
func describeLabelScopes(matches []ScopedLabel) string {
	parts := make([]string, 0, len(matches))
	for _, m := range matches {
		scope := m.Scope
		switch scope {
		case "repo":
			scope = "repository"
		case "org":
			scope = "organization"
		}
		parts = append(parts, fmt.Sprintf("%s label %d", scope, m.ID))
	}
	sort.Strings(parts)
	if len(parts) == 2 {
		return parts[0] + " and " + parts[1]
	}
	return strings.Join(parts, ", ")
}
