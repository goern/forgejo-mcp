// SPDX-License-Identifier: GPL-3.0-or-later

// Package upload resolves attachment content from MCP tool arguments.
package upload

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	// AllowFilePathEnv opts the host into reading upload content off its own
	// filesystem. It is off by default: a `file_path` upload is a file-read
	// primitive handed to whatever drives the MCP client, so a prompt-injected
	// agent could otherwise turn `~/.ssh/id_ed25519` into a public release asset.
	AllowFilePathEnv = "FORGEJO_MCP_ALLOW_FILE_PATH_UPLOAD"

	// UploadRootEnv optionally confines `file_path` reads to one directory.
	// Paths resolving outside it — via `..`, an absolute path, or a symlink —
	// are rejected. Empty means "anywhere the process can read".
	UploadRootEnv = "FORGEJO_MCP_UPLOAD_ROOT"
)

// filePathUploadsEnabled reports whether AllowFilePathEnv is set to a truthy value.
func filePathUploadsEnabled() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(AllowFilePathEnv))) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}

// confineToRoot rejects a resolved path that escapes UploadRootEnv. Both sides
// are symlink-resolved first, so a symlink inside the root cannot point out of it.
func confineToRoot(resolvedPath string) error {
	root := strings.TrimSpace(os.Getenv(UploadRootEnv))
	if root == "" {
		return nil
	}
	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return fmt.Errorf("resolve %s: %w", UploadRootEnv, err)
	}
	realRoot, err := filepath.EvalSymlinks(absoluteRoot)
	if err != nil {
		return fmt.Errorf("resolve %s: %w", UploadRootEnv, err)
	}
	relative, err := filepath.Rel(realRoot, resolvedPath)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return fmt.Errorf("file_path must resolve inside %s (%s)", UploadRootEnv, realRoot)
	}
	return nil
}

// Source describes one attachment upload source.
// Pointers preserve whether an optional MCP argument was supplied.
type Source struct {
	Content  *string
	FilePath *string
	Filename string
}

// SourceFromArguments preserves the presence of optional MCP arguments.
func SourceFromArguments(arguments map[string]any) Source {
	var content *string
	if value, exists := arguments["content"]; exists {
		text, _ := value.(string)
		content = &text
	}
	var filePath *string
	if value, exists := arguments["file_path"]; exists {
		text, _ := value.(string)
		filePath = &text
	}
	filename, _ := arguments["filename"].(string)
	return Source{Content: content, FilePath: filePath, Filename: filename}
}

// Open validates the source and returns its reader and upload filename.
func Open(source Source) (io.ReadCloser, string, error) {
	if (source.Content == nil) == (source.FilePath == nil) {
		return nil, "", fmt.Errorf("exactly one of content or file_path is required")
	}

	if source.Content != nil {
		if source.Filename == "" {
			return nil, "", fmt.Errorf("filename is required when content is used")
		}
		raw, err := base64.StdEncoding.DecodeString(*source.Content)
		if err != nil {
			return nil, "", fmt.Errorf("content must be base64-encoded: %w", err)
		}
		return io.NopCloser(bytes.NewReader(raw)), source.Filename, nil
	}

	if !filePathUploadsEnabled() {
		return nil, "", fmt.Errorf("file_path uploads are disabled on this host; set %s=1 to enable them, or pass base64 content instead", AllowFilePathEnv)
	}
	if *source.FilePath == "" {
		return nil, "", fmt.Errorf("file_path must not be empty")
	}
	absolutePath, err := filepath.Abs(*source.FilePath)
	if err != nil {
		return nil, "", fmt.Errorf("resolve file_path: %w", err)
	}
	// Resolve symlinks before the confinement check so the path we authorise is
	// the path we open.
	if resolved, err := filepath.EvalSymlinks(absolutePath); err == nil {
		absolutePath = resolved
	}
	if err := confineToRoot(absolutePath); err != nil {
		return nil, "", err
	}
	file, err := os.Open(absolutePath)
	if err != nil {
		return nil, "", fmt.Errorf("open file_path: %w", err)
	}
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, "", fmt.Errorf("inspect file_path: %w", err)
	}
	if !info.Mode().IsRegular() {
		_ = file.Close()
		return nil, "", fmt.Errorf("file_path must reference a regular file")
	}

	filename := source.Filename
	if filename == "" {
		filename = filepath.Base(absolutePath)
	}
	return file, filename, nil
}
