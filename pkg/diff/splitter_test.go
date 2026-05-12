package diff

import "testing"

const multiFileDiff = `diff --git a/cmd/main.go b/cmd/main.go
index 1234..5678 100644
--- a/cmd/main.go
+++ b/cmd/main.go
@@ -1,3 +1,4 @@
 package main

+import "fmt"
 func main() {}
diff --git a/README.md b/README.md
index abcd..ef01 100644
--- a/README.md
+++ b/README.md
@@ -10,2 +10,3 @@
 line ten
+line eleven
 line twelve
diff --git a/img.png b/img.png
Binary files a/img.png and b/img.png differ`

func TestFileSlice_MultiFile_SelectsFirst(t *testing.T) {
	out, ok := FileSlice(multiFileDiff, "cmd/main.go")
	if !ok {
		t.Fatal("expected found")
	}
	if !contains(out, "diff --git a/cmd/main.go b/cmd/main.go") {
		t.Fatalf("missing header in slice:\n%s", out)
	}
	if contains(out, "README.md") || contains(out, "img.png") {
		t.Fatalf("slice leaked into other files:\n%s", out)
	}
	if !contains(out, `+import "fmt"`) {
		t.Fatalf("slice missing expected hunk content:\n%s", out)
	}
}

func TestFileSlice_MultiFile_SelectsMiddle(t *testing.T) {
	out, ok := FileSlice(multiFileDiff, "README.md")
	if !ok {
		t.Fatal("expected found")
	}
	if !contains(out, "diff --git a/README.md b/README.md") {
		t.Fatalf("missing header in slice:\n%s", out)
	}
	if contains(out, "cmd/main.go") || contains(out, "img.png") {
		t.Fatalf("slice leaked into other files:\n%s", out)
	}
}

func TestFileSlice_BinaryFile(t *testing.T) {
	out, ok := FileSlice(multiFileDiff, "img.png")
	if !ok {
		t.Fatal("expected found")
	}
	if !contains(out, "Binary files a/img.png and b/img.png differ") {
		t.Fatalf("binary marker missing:\n%s", out)
	}
}

func TestFileSlice_SingleFile(t *testing.T) {
	single := `diff --git a/only.go b/only.go
index 1234..5678 100644
--- a/only.go
+++ b/only.go
@@ -1 +1 @@
-old
+new`
	out, ok := FileSlice(single, "only.go")
	if !ok {
		t.Fatal("expected found")
	}
	if out != single {
		t.Fatalf("single-file slice should equal input. got:\n%s\nwant:\n%s", out, single)
	}
}

func TestFileSlice_Rename_MatchesEitherSide(t *testing.T) {
	rename := `diff --git a/old.go b/new.go
similarity index 95%
rename from old.go
rename to new.go
--- a/old.go
+++ b/new.go
@@ -1 +1 @@
-package old
+package new`

	if _, ok := FileSlice(rename, "old.go"); !ok {
		t.Fatal("expected match on pre-rename path")
	}
	if _, ok := FileSlice(rename, "new.go"); !ok {
		t.Fatal("expected match on post-rename path")
	}
	if _, ok := FileSlice(rename, "neither.go"); ok {
		t.Fatal("did not expect match for unrelated path")
	}
}

func TestFileSlice_NotFound(t *testing.T) {
	out, ok := FileSlice(multiFileDiff, "does/not/exist.go")
	if ok || out != "" {
		t.Fatalf("expected not-found and empty slice, got ok=%v out=%q", ok, out)
	}
}

func TestFileSlice_EmptyInputs(t *testing.T) {
	if _, ok := FileSlice("", "x"); ok {
		t.Fatal("empty diff should yield not-found")
	}
	if _, ok := FileSlice(multiFileDiff, ""); ok {
		t.Fatal("empty path should yield not-found")
	}
}

func TestFileSlice_NoDiffGitMarker(t *testing.T) {
	garbage := "this is not a diff\njust some text\n"
	if _, ok := FileSlice(garbage, "anything"); ok {
		t.Fatal("expected not-found on non-diff input")
	}
}

func contains(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
