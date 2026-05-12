// Package diff offers small helpers for slicing unified-diff strings.
// It exists so MCP tools that return diff payloads can satisfy the
// output-bounding rule (docs/design/output-bounding.md) by handing
// callers just the file they asked for.
package diff

import "strings"

// FileSlice extracts the section of rawDiff that describes the file
// identified by filePath. It returns the section (including its
// `diff --git ...` header line up to but not including the next
// `diff --git ...` line or end of input) and a found flag.
//
// Matching is exact on either the pre-rename ("a/<path>") or
// post-rename ("b/<path>") side, so callers using the post-rename
// filename from list_pull_request_files do not have to special-case
// renames.
//
// If filePath is empty or rawDiff contains no matching section,
// FileSlice returns ("", false).
func FileSlice(rawDiff, filePath string) (string, bool) {
	if filePath == "" || rawDiff == "" {
		return "", false
	}

	const marker = "diff --git "
	lines := strings.Split(rawDiff, "\n")

	startIdx := -1
	for i, line := range lines {
		if !strings.HasPrefix(line, marker) {
			continue
		}
		if matchesPath(line, filePath) {
			startIdx = i
			break
		}
	}
	if startIdx < 0 {
		return "", false
	}

	endIdx := len(lines)
	for i := startIdx + 1; i < len(lines); i++ {
		if strings.HasPrefix(lines[i], marker) {
			endIdx = i
			break
		}
	}

	return strings.Join(lines[startIdx:endIdx], "\n"), true
}

// matchesPath reports whether a `diff --git ...` header line names
// filePath on either the a/ or b/ side. The header format is
// `diff --git a/<old> b/<new>` with `<old>` and `<new>` separated by
// a literal " b/" string. Paths containing spaces are not quoted by
// git unless `core.quotePath` is on; we accept both shapes.
func matchesPath(headerLine, filePath string) bool {
	const prefix = "diff --git "
	rest := strings.TrimPrefix(headerLine, prefix)

	// Find the " b/" separator that splits the two halves. The
	// substring " b/" inside a path would only occur if the file
	// literally contains that sequence, which is vanishingly rare
	// and is the same ambiguity git itself accepts.
	sep := " b/"
	idx := strings.Index(rest, sep)
	if idx < 0 {
		return false
	}
	a := rest[:idx]
	b := rest[idx+len(sep):]

	// a starts with "a/"; trim it. Trim trailing whitespace just in
	// case the line has CR/LF leftovers.
	a = strings.TrimPrefix(a, "a/")
	b = strings.TrimRight(b, " \r")

	return a == filePath || b == filePath
}
