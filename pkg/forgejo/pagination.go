// SPDX-License-Identifier: GPL-3.0-or-later

package forgejo

import (
	"net/http"
	"strconv"
)

// TotalCountHeader is the response header Forgejo sets on paginated
// list/search API endpoints, carrying the total number of items matching
// the query across every page — as opposed to the item count on the
// current page alone.
const TotalCountHeader = "X-Total-Count"

// TotalCount reads and parses the X-Total-Count header. It returns
// (0, false) when the header is absent or does not parse as a
// non-negative integer, so callers can omit the total_count field from
// their response entirely rather than emit a misleading 0
// (docs/design/output-bounding.md sub-rule 3: "Got 4 KB of N" beats
// "got 4 KB" — but only when N is known).
func TotalCount(header http.Header) (int, bool) {
	if header == nil {
		return 0, false
	}
	raw := header.Get(TotalCountHeader)
	if raw == "" {
		return 0, false
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 0 {
		return 0, false
	}
	return n, true
}

// TotalCountPtr is TotalCount adapted for embedding directly into a
// response struct tagged `json:"total_count,omitempty"`: nil when the
// header is absent or unparsable (so the key is omitted from the
// marshaled JSON), a pointer to the parsed value otherwise.
func TotalCountPtr(header http.Header) *int {
	n, ok := TotalCount(header)
	if !ok {
		return nil
	}
	return &n
}
