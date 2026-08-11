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
)

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

	if *source.FilePath == "" {
		return nil, "", fmt.Errorf("file_path must not be empty")
	}
	absolutePath, err := filepath.Abs(*source.FilePath)
	if err != nil {
		return nil, "", fmt.Errorf("resolve file_path: %w", err)
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
