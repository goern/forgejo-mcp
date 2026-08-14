#!/bin/sh
# SPDX-License-Identifier: GPL-3.0-or-later
#
# check-api-path-escaping.sh — reject hand-built raw-HTTP API paths.
#
# The raw-HTTP helpers in pkg/forgejo (DoJSON, DoJSONList, DoAPIRaw,
# DoMultipart) concatenate the path they are given onto the base URL verbatim.
# The SDK client, by contrast, escapes every user-supplied segment for us. So a
# path built with a bare fmt.Sprintf("/repos/%s/%s/...", owner, repo) lets an
# owner or repo containing "/" or "?" escape its segment and retarget the
# request at a different endpoint — which matters most at the DELETE/PATCH
# call sites.
#
# pkg/forgejo.APIPath() escapes every segment. This checker fails the build if
# a new call site goes back to formatting an API path by hand, i.e. any
# fmt.Sprintf whose format string starts with a literal "/" — that is the shape
# of an API path, and it is never the shape of an APIPath() call.
#
# Exit 0 when clean, 1 when a hand-built path is found.

set -eu

cd "$(dirname "$0")/../.."

# Non-test Go sources under operation/ and pkg/. Vendored code is not ours.
files=$(find operation pkg -name '*.go' ! -name '*_test.go' | sort)
[ -n "$files" ] || exit 0

# shellcheck disable=SC2086
hits=$(grep -n 'fmt\.Sprintf("/' $files || true)

if [ -n "$hits" ]; then
	echo "error: API path built with fmt.Sprintf instead of forgejo.APIPath()." >&2
	echo "" >&2
	echo "$hits" >&2
	echo "" >&2
	echo "Path segments interpolated this way are not escaped, so an owner or" >&2
	echo "repo containing '/' or '?' can retarget the request at another" >&2
	echo "endpoint. Use forgejo.APIPath(\"repos\", owner, repo, ...) and append" >&2
	echo "any query string to its result." >&2
	exit 1
fi

exit 0
