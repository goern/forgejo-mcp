// SPDX-License-Identifier: GPL-3.0-or-later

package upload

import (
	"encoding/base64"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func stringPointer(value string) *string {
	return &value
}

func TestOpenBase64(t *testing.T) {
	content := base64.StdEncoding.EncodeToString([]byte("payload"))
	reader, filename, err := Open(Source{Content: &content, Filename: "file.bin"})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer reader.Close()
	got, _ := io.ReadAll(reader)
	if string(got) != "payload" || filename != "file.bin" {
		t.Fatalf("got payload=%q filename=%q", got, filename)
	}
}

func TestOpenEmptyBase64(t *testing.T) {
	reader, _, err := Open(Source{Content: stringPointer(""), Filename: "empty.txt"})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer reader.Close()
	got, _ := io.ReadAll(reader)
	if len(got) != 0 {
		t.Fatalf("expected empty payload, got %q", got)
	}
}

func TestOpenFilePath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "payload.txt")
	if err := os.WriteFile(path, []byte("from file"), 0o600); err != nil {
		t.Fatal(err)
	}
	reader, filename, err := Open(Source{FilePath: &path})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer reader.Close()
	got, _ := io.ReadAll(reader)
	if string(got) != "from file" || filename != "payload.txt" {
		t.Fatalf("got payload=%q filename=%q", got, filename)
	}
}

func TestOpenRelativeFilePathAndFilenameOverride(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "payload.txt")
	if err := os.WriteFile(path, []byte("relative"), 0o600); err != nil {
		t.Fatal(err)
	}
	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(previous) })

	relative := "payload.txt"
	reader, filename, err := Open(Source{FilePath: &relative, Filename: "renamed.txt"})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer reader.Close()
	if filename != "renamed.txt" {
		t.Fatalf("filename: %q", filename)
	}
}

func TestOpenRejectsInvalidSources(t *testing.T) {
	validPath := filepath.Join(t.TempDir(), "file.txt")
	if err := os.WriteFile(validPath, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name   string
		source Source
		want   string
	}{
		{"neither", Source{}, "exactly one"},
		{"both including empty content", Source{Content: stringPointer(""), FilePath: &validPath}, "exactly one"},
		{"empty path", Source{FilePath: stringPointer("")}, "must not be empty"},
		{"directory", Source{FilePath: stringPointer(t.TempDir())}, "regular file"},
		{"missing", Source{FilePath: stringPointer(filepath.Join(t.TempDir(), "missing"))}, "open file_path"},
		{"base64 without filename", Source{Content: stringPointer("eA==")}, "filename is required"},
		{"invalid base64", Source{Content: stringPointer("!!!"), Filename: "x"}, "base64-encoded"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			reader, _, err := Open(test.source)
			if reader != nil {
				_ = reader.Close()
			}
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("expected %q error, got %v", test.want, err)
			}
		})
	}
}
