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

// Sentinel returns a truncation marker string when the list was truncated, or empty string otherwise.
func (b BoundedResult) Sentinel() string {
	if !b.Truncated {
		return ""
	}
	return fmt.Sprintf("[truncated: %d of %d items shown. Use %s tool to fetch more.]", b.Cap, b.Total, b.ListTool)
}
