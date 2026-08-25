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

// allowFilePathUploads opts the test into the host-side file read that
// AllowFilePathEnv gates. Tests that assert the gate itself must not call it.
func allowFilePathUploads(t *testing.T) {
	t.Helper()
	t.Setenv(AllowFilePathEnv, "1")
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
	allowFilePathUploads(t)
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
	allowFilePathUploads(t)
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

func TestOpenRejectsFilePathWhenNotEnabled(t *testing.T) {
	path := filepath.Join(t.TempDir(), "payload.txt")
	if err := os.WriteFile(path, []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}

	for _, value := range []string{"", "0", "false", "no"} {
		t.Run("env="+value, func(t *testing.T) {
			t.Setenv(AllowFilePathEnv, value)
			reader, _, err := Open(Source{FilePath: &path})
			if reader != nil {
				_ = reader.Close()
			}
			if err == nil || !strings.Contains(err.Error(), AllowFilePathEnv) {
				t.Fatalf("expected the gate to reject file_path, got %v", err)
			}
		})
	}
}

// Base64 uploads are unaffected by the gate — it only guards host file reads.
func TestOpenBase64IgnoresGate(t *testing.T) {
	t.Setenv(AllowFilePathEnv, "")
	content := base64.StdEncoding.EncodeToString([]byte("payload"))
	reader, _, err := Open(Source{Content: &content, Filename: "file.bin"})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	_ = reader.Close()
}

func TestOpenConfinesToUploadRoot(t *testing.T) {
	allowFilePathUploads(t)
	root := t.TempDir()
	inside := filepath.Join(root, "inside.txt")
	if err := os.WriteFile(inside, []byte("ok"), 0o600); err != nil {
		t.Fatal(err)
	}
	outsideDir := t.TempDir()
	outside := filepath.Join(outsideDir, "outside.txt")
	if err := os.WriteFile(outside, []byte("nope"), 0o600); err != nil {
		t.Fatal(err)
	}
	// A symlink inside the root must not become a way out of it.
	escape := filepath.Join(root, "escape.txt")
	if err := os.Symlink(outside, escape); err != nil {
		t.Fatal(err)
	}
	t.Setenv(UploadRootEnv, root)

	reader, filename, err := Open(Source{FilePath: &inside})
	if err != nil {
		t.Fatalf("path inside the root: %v", err)
	}
	_ = reader.Close()
	if filename != "inside.txt" {
		t.Fatalf("filename: %q", filename)
	}

	for name, path := range map[string]string{"outside": outside, "symlink out": escape} {
		t.Run(name, func(t *testing.T) {
			reader, _, err := Open(Source{FilePath: &path})
			if reader != nil {
				_ = reader.Close()
			}
			if err == nil || !strings.Contains(err.Error(), UploadRootEnv) {
				t.Fatalf("expected confinement error, got %v", err)
			}
		})
	}
}

// A sibling directory whose name merely starts with the root's name (e.g.
// "updir-evil" next to "updir") must not pass the confinement check. A naive
// strings.HasPrefix(path, root) implementation would wrongly accept it;
// confineToRoot uses filepath.Rel, which does not.
func TestOpenConfinesToUploadRootRejectsSiblingPrefixDir(t *testing.T) {
	allowFilePathUploads(t)
	parent := t.TempDir()
	root := filepath.Join(parent, "updir")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	siblingDir := filepath.Join(parent, "updir-evil")
	if err := os.Mkdir(siblingDir, 0o700); err != nil {
		t.Fatal(err)
	}
	siblingFile := filepath.Join(siblingDir, "secret.txt")
	if err := os.WriteFile(siblingFile, []byte("nope"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(UploadRootEnv, root)

	reader, _, err := Open(Source{FilePath: &siblingFile})
	if reader != nil {
		_ = reader.Close()
	}
	if err == nil || !strings.Contains(err.Error(), UploadRootEnv) {
		t.Fatalf("expected sibling-prefix dir %q to be rejected against root %q, got %v", siblingDir, root, err)
	}
}

func TestOpenRejectsInvalidSources(t *testing.T) {
	allowFilePathUploads(t)
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
