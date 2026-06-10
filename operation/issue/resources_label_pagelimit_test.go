// SPDX-License-Identifier: GPL-3.0-or-later

package issue

import (
	"testing"

	"codeberg.org/goern/forgejo-mcp/v2/operation/resource"
	"github.com/mark3labs/mcp-go/mcp"
)

func readReq(uri string) mcp.ReadResourceRequest {
	var req mcp.ReadResourceRequest
	req.Params.URI = uri
	return req
}

func TestPageLimitDefaults(t *testing.T) {
	page, limit := pageLimit(readReq("forgejo://repo/o/r/labels"))
	if page != 1 || limit != resource.EmbeddedListCap {
		t.Fatalf("got page=%d limit=%d", page, limit)
	}
}

func TestPageLimitFromQuery(t *testing.T) {
	page, limit := pageLimit(readReq("forgejo://repo/o/r/labels?page=3&limit=5"))
	if page != 3 || limit != 5 {
		t.Fatalf("got page=%d limit=%d", page, limit)
	}
}

func TestPageLimitClampsToCap(t *testing.T) {
	_, limit := pageLimit(readReq("forgejo://org/o/labels?limit=999"))
	if limit != resource.EmbeddedListCap {
		t.Fatalf("got limit=%d, want cap %d", limit, resource.EmbeddedListCap)
	}
}

func TestPageLimitIgnoresMalformed(t *testing.T) {
	page, limit := pageLimit(readReq("forgejo://org/o/labels?page=abc&limit=-2"))
	if page != 1 || limit != resource.EmbeddedListCap {
		t.Fatalf("got page=%d limit=%d", page, limit)
	}
}
