package resource

import "fmt"

// EmbeddedListCap is the default maximum number of items embedded in a resource response.
const EmbeddedListCap = 30

// BoundedResult holds a capped slice of items and metadata for sentinel generation.
type BoundedResult struct {
	Items     []string
	Total     int
	Cap       int
	ListTool  string
	Truncated bool
}

// Bounded returns a BoundedResult capping items at cap (use EmbeddedListCap for default).
// listTool names the tool the caller should use to fetch more items.
func Bounded(items []string, cap int, listTool string) BoundedResult {
	total := len(items)
	truncated := total > cap
	shown := items
	if truncated {
		shown = items[:cap]
	}
	return BoundedResult{
		Items:     shown,
		Total:     total,
		Cap:       cap,
		ListTool:  listTool,
		Truncated: truncated,
	}
}

// WithMoreRemaining marks the result truncated because the server said more
// rows exist, rather than because an over-fetch handed back one item too many.
//
// It exists for paged resources. Detecting truncation by fetching cap+1 items
// only works when the fetch is not also the page: the upstream offset is
// (page-1)*PageSize, so a PageSize of cap+1 that reports only cap rows makes
// page N+1 start one row past the end of page N and the row in between is
// unreachable. Such resources must request exactly cap rows and take "more
// exists" from the response's Link header (forgejo_sdk.Response.NextPage).
//
// total is the server's authoritative count of matching rows — Forgejo reports
// it in X-Total-Count — or 0 when it did not say. A total that does not exceed
// the rows in hand is treated as unknown rather than trusted, so the sentinel
// never claims a count it cannot support.
func (b BoundedResult) WithMoreRemaining(total int) BoundedResult {
	b.Truncated = true
	if total > len(b.Items) {
		b.Total = total
	} else {
		b.Total = 0
	}
	return b
}

// Sentinel returns a truncation marker string when the list was truncated, or empty string otherwise.
func (b BoundedResult) Sentinel() string {
	if !b.Truncated {
		return ""
	}
	if b.Total <= b.Cap {
		// Truncation known, total not. Saying "N of N items shown" here would
		// be worse than saying nothing about the total.
		return fmt.Sprintf("[truncated: %d items shown, more remain. Use %s tool to fetch more.]", b.Cap, b.ListTool)
	}
	return fmt.Sprintf("[truncated: %d of %d items shown. Use %s tool to fetch more.]", b.Cap, b.Total, b.ListTool)
}
