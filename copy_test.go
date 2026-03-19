package copy

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

var testData = []byte("test file data")

// createTree builds a source tree inside dir with known folders, files,
// and symlinks for use in tests.
func createTree(t *testing.T, dir string) {
	t.Helper()

	dirs := []string{
		"folder1",
		"folder1/folder2",
		"folder1/folder2_1",
		"folder1/folder2/folder3",
	}

	files := []string{
		"file1",
		"folder1/folder2/file2",
		"folder1/folder2/folder3/file3",
	}

	symlinks := [][2]string{
		{"folder1/folder2", "folder2_link"},
		{"folder1/folder2/folder3/file3", "file3_link"},
	}

	for _, d := range dirs {
		if err := os.MkdirAll(filepath.Join(dir, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	for _, f := range files {
		p := filepath.Join(dir, f)
		if err := os.WriteFile(p, testData, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	for _, s := range symlinks {
		src := filepath.Join(dir, s[0])
		dst := filepath.Join(dir, s[1])
		if err := os.Symlink(src, dst); err != nil {
			t.Fatal(err)
		}
	}
}

// assertFileContent checks that the file at path has exactly the expected bytes.
func assertFileContent(t *testing.T, path string, want []byte) {
	t.Helper()

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("cannot read %s: %v", path, err)
	}

	if !bytes.Equal(want, got) {
		t.Errorf("content mismatch at %s: want %q, got %q", path, want, got)
	}
}

// assertExists checks that the path exists.
func assertExists(t *testing.T, path string) {
	t.Helper()

	if _, err := os.Stat(path); err != nil {
		t.Errorf("expected %s to exist: %v", path, err)
	}
}

// assertNotExists checks that the path does NOT exist.
func assertNotExists(t *testing.T, path string) {
	t.Helper()

	if _, err := os.Stat(path); err == nil {
		t.Errorf("expected %s to not exist, but it does", path)
	} else if !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("unexpected error checking %s: %v", path, err)
	}
}

// assertSymlink checks that the path is a symlink.
func assertSymlink(t *testing.T, path string) {
	t.Helper()

	info, err := os.Lstat(path)
	if err != nil {
		t.Fatalf("cannot lstat %s: %v", path, err)
	}

	if info.Mode()&os.ModeSymlink == 0 {
		t.Errorf("expected %s to be a symlink", path)
	}
}

// assertNotSymlink checks that the path exists and is NOT a symlink.
func assertNotSymlink(t *testing.T, path string) {
	t.Helper()

	info, err := os.Lstat(path)
	if err != nil {
		t.Fatalf("cannot lstat %s: %v", path, err)
	}

	if info.Mode()&os.ModeSymlink != 0 {
		t.Errorf("expected %s to not be a symlink", path)
	}
}

// ── File Copy Tests ─────────────────────────────────────────────────────

func TestCopyFile(t *testing.T) {
	src := t.TempDir()
	srcFile := filepath.Join(src, "data.txt")
	if err := os.WriteFile(srcFile, testData, 0o644); err != nil {
		t.Fatal(err)
	}

	dst := filepath.Join(t.TempDir(), "copied.txt")

	if err := Copy(t.Context(), srcFile, dst); err != nil {
		t.Fatal(err)
	}

	assertFileContent(t, dst, testData)
}

func TestCopyFileIntoDirectory(t *testing.T) {
	src := t.TempDir()
	srcFile := filepath.Join(src, "data.txt")
	if err := os.WriteFile(srcFile, testData, 0o644); err != nil {
		t.Fatal(err)
	}

	dst := filepath.Join(t.TempDir(), "target")
	if err := os.Mkdir(dst, 0o755); err != nil {
		t.Fatal(err)
	}

	// Trailing slash means "copy into this directory".
	if err := Copy(t.Context(), srcFile, dst+"/", Force); err != nil {
		t.Fatal(err)
	}

	assertFileContent(t, filepath.Join(dst, "data.txt"), testData)
}

func TestCopyFileForceOverwrite(t *testing.T) {
	src := t.TempDir()
	srcFile := filepath.Join(src, "data.txt")
	if err := os.WriteFile(srcFile, testData, 0o644); err != nil {
		t.Fatal(err)
	}

	dst := t.TempDir()
	dstFile := filepath.Join(dst, "data.txt")
	if err := os.WriteFile(dstFile, []byte("old data"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := Copy(t.Context(), srcFile, dstFile, Force); err != nil {
		t.Fatal(err)
	}

	assertFileContent(t, dstFile, testData)
}

func TestCopyFilePreservesPermissions(t *testing.T) {
	src := t.TempDir()
	srcFile := filepath.Join(src, "script.sh")
	if err := os.WriteFile(srcFile, []byte("#!/bin/sh"), 0o755); err != nil {
		t.Fatal(err)
	}

	dstFile := filepath.Join(t.TempDir(), "script.sh")

	if err := Copy(t.Context(), srcFile, dstFile); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(dstFile)
	if err != nil {
		t.Fatal(err)
	}

	// Check that the executable bit is preserved.
	if info.Mode().Perm()&0o100 == 0 {
		t.Errorf("expected executable bit, got %o", info.Mode().Perm())
	}
}

func TestCopyEmptyFile(t *testing.T) {
	src := t.TempDir()
	srcFile := filepath.Join(src, "empty.txt")
	if err := os.WriteFile(srcFile, nil, 0o644); err != nil {
		t.Fatal(err)
	}

	dstFile := filepath.Join(t.TempDir(), "empty.txt")

	if err := Copy(t.Context(), srcFile, dstFile); err != nil {
		t.Fatal(err)
	}

	assertFileContent(t, dstFile, nil)
}

func TestCopyLargeFile(t *testing.T) {
	// Create a file that is larger than the default buffer (64KB)
	// to ensure multi-read works correctly.
	size := 256 * 1024
	data := make([]byte, size)
	for i := range data {
		data[i] = byte(i % 251) // prime-based pattern.
	}

	src := t.TempDir()
	srcFile := filepath.Join(src, "large.bin")
	if err := os.WriteFile(srcFile, data, 0o644); err != nil {
		t.Fatal(err)
	}

	dstFile := filepath.Join(t.TempDir(), "large.bin")

	if err := Copy(t.Context(), srcFile, dstFile); err != nil {
		t.Fatal(err)
	}

	assertFileContent(t, dstFile, data)
}

func TestCopyFileCustomBufferSize(t *testing.T) {
	src := t.TempDir()
	srcFile := filepath.Join(src, "data.txt")
	data := bytes.Repeat([]byte("abcdefgh"), 512) // 4KB.
	if err := os.WriteFile(srcFile, data, 0o644); err != nil {
		t.Fatal(err)
	}

	dstFile := filepath.Join(t.TempDir(), "data.txt")

	// Use a very small buffer (128 bytes) to force many reads.
	if err := Copy(t.Context(), srcFile, dstFile, WithBufferSize(128)); err != nil {
		t.Fatal(err)
	}

	assertFileContent(t, dstFile, data)
}

func TestCopyFileNegativeBufferFallsBackToDefault(t *testing.T) {
	src := t.TempDir()
	srcFile := filepath.Join(src, "data.txt")
	if err := os.WriteFile(srcFile, testData, 0o644); err != nil {
		t.Fatal(err)
	}

	dstFile := filepath.Join(t.TempDir(), "data.txt")

	// Negative buffer size should use the default.
	if err := Copy(t.Context(), srcFile, dstFile, WithBufferSize(-1)); err != nil {
		t.Fatal(err)
	}

	assertFileContent(t, dstFile, testData)
}

func TestCopyFileCreatesIntermediateDirectories(t *testing.T) {
	src := t.TempDir()
	srcFile := filepath.Join(src, "data.txt")
	if err := os.WriteFile(srcFile, testData, 0o644); err != nil {
		t.Fatal(err)
	}

	// Destination with non-existent parent directories.
	dstFile := filepath.Join(t.TempDir(), "a", "b", "c", "data.txt")

	if err := Copy(t.Context(), srcFile, dstFile); err != nil {
		t.Fatal(err)
	}

	assertFileContent(t, dstFile, testData)
}

// ── Directory (Tree) Copy Tests ─────────────────────────────────────────

func TestCopyTree(t *testing.T) {
	src := t.TempDir()
	createTree(t, src)

	dst := filepath.Join(t.TempDir(), "copy")

	if err := Copy(t.Context(), src, dst); err != nil {
		t.Fatal(err)
	}

	// Verify structure.
	for _, d := range []string{
		"folder1", "folder1/folder2", "folder1/folder2_1",
		"folder1/folder2/folder3",
	} {
		assertExists(t, filepath.Join(dst, d))
	}

	// Verify file contents.
	for _, f := range []string{
		"file1", "folder1/folder2/file2",
		"folder1/folder2/folder3/file3",
	} {
		assertFileContent(t, filepath.Join(dst, f), testData)
	}

	// Symlinks should be resolved by default (not remain as symlinks).
	assertNotSymlink(t, filepath.Join(dst, "folder2_link"))
	assertNotSymlink(t, filepath.Join(dst, "file3_link"))
}

func TestCopyTreeWithCustomBuffer(t *testing.T) {
	src := t.TempDir()
	createTree(t, src)

	dst := filepath.Join(t.TempDir(), "copy")

	if err := Copy(t.Context(), src, dst, WithBufferSize(1024)); err != nil {
		t.Fatal(err)
	}

	assertFileContent(t, filepath.Join(dst, "file1"), testData)
	assertFileContent(t, filepath.Join(dst, "folder1/folder2/file2"), testData)
}

func TestCopyEmptyDirectory(t *testing.T) {
	src := t.TempDir()
	// src is already an empty directory.
	dst := filepath.Join(t.TempDir(), "empty_copy")

	if err := Copy(t.Context(), src, dst); err != nil {
		t.Fatal(err)
	}

	assertExists(t, dst)

	entries, err := os.ReadDir(dst)
	if err != nil {
		t.Fatal(err)
	}

	if len(entries) != 0 {
		t.Errorf("expected empty directory, got %d entries", len(entries))
	}
}

func TestCopyTreeIntoDirectory(t *testing.T) {
	src := t.TempDir()
	createTree(t, src)

	// Trailing slash: should place tree under dst with source basename.
	dst := filepath.Join(t.TempDir(), "target")
	if err := os.Mkdir(dst, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := Copy(t.Context(), src, dst+"/", Force); err != nil {
		t.Fatal(err)
	}

	base := filepath.Base(src)
	assertExists(t, filepath.Join(dst, base, "file1"))
	assertExists(t, filepath.Join(dst, base, "folder1", "folder2"))
}

// ── Symlink Tests ───────────────────────────────────────────────────────

func TestCopySymlinkFollow(t *testing.T) {
	src := t.TempDir()
	createTree(t, src)

	dst := filepath.Join(t.TempDir(), "follow_link.txt")

	// Copy the file symlink without NoFollow → should copy contents.
	if err := Copy(t.Context(), filepath.Join(src, "file3_link"), dst); err != nil {
		t.Fatal(err)
	}

	assertNotSymlink(t, dst)
	assertFileContent(t, dst, testData)
}

func TestCopySymlinkNoFollow(t *testing.T) {
	src := t.TempDir()
	createTree(t, src)

	dst := filepath.Join(t.TempDir(), "keep_link.txt")

	// Copy the file symlink with NoFollow → should create a symlink.
	if err := Copy(
		t.Context(),
		filepath.Join(src, "file3_link"),
		dst,
		WithNoFollow,
	); err != nil {
		t.Fatal(err)
	}

	assertSymlink(t, dst)

	// Symlink should resolve to the original target.
	target, err := filepath.EvalSymlinks(dst)
	if err != nil {
		t.Fatal(err)
	}

	origTarget, err := filepath.EvalSymlinks(filepath.Join(src, "file3_link"))
	if err != nil {
		t.Fatal(err)
	}

	if target != origTarget {
		t.Errorf("symlink target mismatch: want %s, got %s", origTarget, target)
	}
}

func TestCopyTreeNoFollow(t *testing.T) {
	src := t.TempDir()
	createTree(t, src)

	dst := filepath.Join(t.TempDir(), "copy")

	if err := Copy(t.Context(), src, dst, WithNoFollow); err != nil {
		t.Fatal(err)
	}

	// Regular files/dirs should exist.
	assertExists(t, filepath.Join(dst, "file1"))
	assertExists(t, filepath.Join(dst, "folder1", "folder2"))

	// Symlinks should remain as symlinks.
	assertSymlink(t, filepath.Join(dst, "folder2_link"))
	assertSymlink(t, filepath.Join(dst, "file3_link"))
}

func TestCopyDirSymlinkFollow(t *testing.T) {
	src := t.TempDir()
	createTree(t, src)

	// Copy a directory symlink with follow → copies the resolved directory.
	dst := filepath.Join(t.TempDir(), "resolved_dir")

	if err := Copy(
		t.Context(), filepath.Join(src, "folder2_link"), dst,
	); err != nil {
		t.Fatal(err)
	}

	assertNotSymlink(t, dst)
	assertExists(t, filepath.Join(dst, "file2"))
	assertExists(t, filepath.Join(dst, "folder3"))
	assertFileContent(t, filepath.Join(dst, "file2"), testData)
}

func TestCopyDirSymlinkNoFollow(t *testing.T) {
	src := t.TempDir()
	createTree(t, src)

	dst := filepath.Join(t.TempDir(), "link_dir")

	if err := Copy(
		t.Context(), filepath.Join(src, "folder2_link"), dst, WithNoFollow,
	); err != nil {
		t.Fatal(err)
	}

	assertSymlink(t, dst)
}

// ── Exclude Tests ───────────────────────────────────────────────────────

func TestExcludeFile(t *testing.T) {
	src := t.TempDir()
	createTree(t, src)

	dst := filepath.Join(t.TempDir(), "copy")

	exclude := func(s string) bool {
		return filepath.Base(s) == "file1"
	}

	if err := Copy(t.Context(), src, dst, WithExcludeFunc(exclude)); err != nil {
		t.Fatal(err)
	}

	assertNotExists(t, filepath.Join(dst, "file1"))
	// Other files should still be copied.
	assertFileContent(t, filepath.Join(dst, "folder1/folder2/file2"), testData)
}

func TestExcludeDirectory(t *testing.T) {
	src := t.TempDir()
	createTree(t, src)

	dst := filepath.Join(t.TempDir(), "copy")

	exclude := func(s string) bool {
		return filepath.Base(s) == "folder3"
	}

	if err := Copy(t.Context(), src, dst, WithExcludeFunc(exclude)); err != nil {
		t.Fatal(err)
	}

	// The excluded directory and all its children should not exist.
	assertNotExists(t, filepath.Join(dst, "folder1/folder2/folder3"))
	assertNotExists(t, filepath.Join(dst, "folder1/folder2/folder3/file3"))

	// Other content should still be present.
	assertExists(t, filepath.Join(dst, "folder1/folder2"))
	assertFileContent(t, filepath.Join(dst, "folder1/folder2/file2"), testData)
}

func TestExcludeSingleFile(t *testing.T) {
	src := t.TempDir()
	srcFile := filepath.Join(src, "data.txt")
	if err := os.WriteFile(srcFile, testData, 0o644); err != nil {
		t.Fatal(err)
	}

	dst := filepath.Join(t.TempDir(), "copy.txt")

	exclude := func(s string) bool { return true }

	if err := Copy(t.Context(), srcFile, dst, WithExcludeFunc(exclude)); err != nil {
		t.Fatal(err)
	}

	assertNotExists(t, dst)
}

func TestExcludeEverything(t *testing.T) {
	src := t.TempDir()
	createTree(t, src)

	dst := filepath.Join(t.TempDir(), "copy")

	exclude := func(string) bool { return true }

	if err := Copy(t.Context(), src, dst, WithExcludeFunc(exclude)); err != nil {
		t.Fatal(err)
	}

	// The root itself gets excluded, so nothing should be created.
	assertNotExists(t, dst)
}

func TestExcludeWithNoFollow(t *testing.T) {
	src := t.TempDir()
	createTree(t, src)

	dst := filepath.Join(t.TempDir(), "copy")

	exclude := func(s string) bool {
		return filepath.Base(s) == "folder3"
	}

	if err := Copy(
		t.Context(), src, dst, WithNoFollow, WithExcludeFunc(exclude),
	); err != nil {
		t.Fatal(err)
	}

	assertNotExists(t, filepath.Join(dst, "folder1/folder2/folder3"))
	assertExists(t, filepath.Join(dst, "folder1"))

	// Symlinks that don't point to excluded content should remain.
	assertSymlink(t, filepath.Join(dst, "folder2_link"))
}

// ── Error Path Tests ────────────────────────────────────────────────────

func TestCopySameFile(t *testing.T) {
	f := filepath.Join(t.TempDir(), "same.txt")
	if err := os.WriteFile(f, testData, 0o644); err != nil {
		t.Fatal(err)
	}

	err := Copy(t.Context(), f, f)
	if err == nil {
		t.Fatal("expected ErrSame, got nil")
	}

	if !errors.Is(err, ErrSame) {
		t.Fatalf("expected ErrSame, got: %v", err)
	}
}

func TestCopyAlreadyExistsWithoutForce(t *testing.T) {
	srcFile := filepath.Join(t.TempDir(), "src.txt")
	if err := os.WriteFile(srcFile, testData, 0o644); err != nil {
		t.Fatal(err)
	}

	dstFile := filepath.Join(t.TempDir(), "dst.txt")
	if err := os.WriteFile(dstFile, []byte("existing"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := Copy(t.Context(), srcFile, dstFile)
	if err == nil {
		t.Fatal("expected fs.ErrExist, got nil")
	}

	if !errors.Is(err, fs.ErrExist) {
		t.Fatalf("expected fs.ErrExist, got: %v", err)
	}
}

func TestCopySourceNotExists(t *testing.T) {
	dst := filepath.Join(t.TempDir(), "dst.txt")

	err := Copy(t.Context(), "/nonexistent/path/file.txt", dst)
	if err == nil {
		t.Fatal("expected error for non-existent source, got nil")
	}
}

func TestCopyBrokenSymlink(t *testing.T) {
	dir := t.TempDir()

	link := filepath.Join(dir, "broken_link")
	if err := os.Symlink("/nonexistent/target", link); err != nil {
		t.Fatal(err)
	}

	dst := filepath.Join(t.TempDir(), "out.txt")

	// Without NoFollow, the broken symlink should fail on EvalSymlinks.
	err := Copy(t.Context(), link, dst)
	if err == nil {
		t.Fatal("expected error for broken symlink, got nil")
	}
}

func TestCopyBrokenSymlinkNoFollow(t *testing.T) {
	dir := t.TempDir()

	link := filepath.Join(dir, "broken_link")
	target := "/nonexistent/target"
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}

	dst := filepath.Join(t.TempDir(), "out_link")

	// With NoFollow, Copy no longer calls EvalSymlinks on the source, so a
	// broken symlink can be successfully recreated at the destination.
	if err := Copy(t.Context(), link, dst, WithNoFollow); err != nil {
		t.Fatal(err)
	}

	assertSymlink(t, dst)

	// The new symlink should point to the absolute path of the source symlink
	// itself (since Copy passes filepath.Abs(src) to os.Symlink).
	got, err := os.Readlink(dst)
	if err != nil {
		t.Fatal(err)
	}

	want, err := filepath.Abs(link)
	if err != nil {
		t.Fatal(err)
	}

	if got != want {
		t.Errorf("symlink target: want %s, got %s", want, got)
	}
}

func TestCopyContextCancelledBefore(t *testing.T) {
	src := t.TempDir()
	srcFile := filepath.Join(src, "data.txt")
	if err := os.WriteFile(srcFile, testData, 0o644); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(t.Context())
	cancel() // Cancel immediately.

	dst := filepath.Join(t.TempDir(), "data.txt")

	err := Copy(ctx, srcFile, dst)
	if err == nil {
		t.Fatal("expected context error, got nil")
	}

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got: %v", err)
	}
}

func TestCopyTreeContextCancelledBefore(t *testing.T) {
	src := t.TempDir()
	createTree(t, src)

	ctx, cancel := context.WithCancel(t.Context())
	cancel() // Cancel immediately.

	dst := filepath.Join(t.TempDir(), "copy")

	err := Copy(ctx, src, dst)
	if err == nil {
		t.Fatal("expected context error, got nil")
	}

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got: %v", err)
	}
}

// ── Combined Options Tests ──────────────────────────────────────────────

func TestCopyMultipleOptions(t *testing.T) {
	src := t.TempDir()
	createTree(t, src)

	dst := filepath.Join(t.TempDir(), "copy")

	// Combine Force + NoFollow + exclude + custom buffer.
	if err := Copy(
		t.Context(), src, dst,
		Force,
		WithNoFollow,
		WithBufferSize(512),
		WithExcludeFunc(func(s string) bool {
			return filepath.Base(s) == "folder2_1"
		}),
	); err != nil {
		t.Fatal(err)
	}

	assertNotExists(t, filepath.Join(dst, "folder1/folder2_1"))
	assertExists(t, filepath.Join(dst, "file1"))
	assertSymlink(t, filepath.Join(dst, "folder2_link"))
}

func TestCopyForceOverwriteTree(t *testing.T) {
	src := t.TempDir()
	createTree(t, src)

	dst := filepath.Join(t.TempDir(), "copy")

	// First copy.
	if err := Copy(t.Context(), src, dst); err != nil {
		t.Fatal(err)
	}

	// Modify a file in source.
	newData := []byte("updated content")
	if err := os.WriteFile(
		filepath.Join(src, "file1"), newData, 0o644,
	); err != nil {
		t.Fatal(err)
	}

	// Second copy with Force.
	if err := Copy(t.Context(), src, dst, Force); err != nil {
		t.Fatal(err)
	}

	assertFileContent(t, filepath.Join(dst, "file1"), newData)
}

// ── Edge Case Tests ─────────────────────────────────────────────────────

func TestCopyFileZeroBufferFallback(t *testing.T) {
	src := t.TempDir()
	srcFile := filepath.Join(src, "data.txt")
	if err := os.WriteFile(srcFile, testData, 0o644); err != nil {
		t.Fatal(err)
	}

	dstFile := filepath.Join(t.TempDir(), "data.txt")

	// Zero buffer size should use default.
	if err := Copy(t.Context(), srcFile, dstFile, WithBufferSize(0)); err != nil {
		t.Fatal(err)
	}

	assertFileContent(t, dstFile, testData)
}

func TestCopySingleByteBuffer(t *testing.T) {
	src := t.TempDir()
	srcFile := filepath.Join(src, "data.txt")
	if err := os.WriteFile(srcFile, testData, 0o644); err != nil {
		t.Fatal(err)
	}

	dstFile := filepath.Join(t.TempDir(), "data.txt")

	// 1-byte buffer: extreme stress test of the copy loop.
	if err := Copy(t.Context(), srcFile, dstFile, WithBufferSize(1)); err != nil {
		t.Fatal(err)
	}

	assertFileContent(t, dstFile, testData)
}

func TestCopyTreeDirectoryPermissions(t *testing.T) {
	src := t.TempDir()
	subDir := filepath.Join(src, "restricted")
	if err := os.Mkdir(subDir, 0o700); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(
		filepath.Join(subDir, "secret.txt"), testData, 0o600,
	); err != nil {
		t.Fatal(err)
	}

	dst := filepath.Join(t.TempDir(), "copy")

	if err := Copy(t.Context(), src, dst); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(filepath.Join(dst, "restricted"))
	if err != nil {
		t.Fatal(err)
	}

	if info.Mode().Perm() != 0o700 {
		t.Errorf("expected perm 0700, got %o", info.Mode().Perm())
	}

	assertFileContent(t, filepath.Join(dst, "restricted/secret.txt"), testData)
}

func TestCopyTreeDeeplyNested(t *testing.T) {
	src := t.TempDir()

	deep := src
	for i := range 10 {
		deep = filepath.Join(deep, string(rune('a'+i)))
		if err := os.Mkdir(deep, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	if err := os.WriteFile(
		filepath.Join(deep, "deep.txt"), testData, 0o644,
	); err != nil {
		t.Fatal(err)
	}

	dst := filepath.Join(t.TempDir(), "copy")

	if err := Copy(t.Context(), src, dst); err != nil {
		t.Fatal(err)
	}

	deepDst := dst
	for i := range 10 {
		deepDst = filepath.Join(deepDst, string(rune('a'+i)))
	}

	assertFileContent(t, filepath.Join(deepDst, "deep.txt"), testData)
}

func TestCopyExcludeReceivesAbsolutePaths(t *testing.T) {
	src := t.TempDir()
	createTree(t, src)

	dst := filepath.Join(t.TempDir(), "copy")

	var seen []string
	exclude := func(s string) bool {
		seen = append(seen, s)
		return false
	}

	if err := Copy(t.Context(), src, dst, WithExcludeFunc(exclude)); err != nil {
		t.Fatal(err)
	}

	for _, p := range seen {
		if !filepath.IsAbs(p) {
			t.Errorf("exclude function received non-absolute path: %s", p)
		}
	}
}

// ── Into Option Tests ───────────────────────────────────────────────────

func TestCopyFileInto(t *testing.T) {
	src := t.TempDir()
	srcFile := filepath.Join(src, "data.txt")
	if err := os.WriteFile(srcFile, testData, 0o644); err != nil {
		t.Fatal(err)
	}

	dst := t.TempDir()

	if err := Copy(t.Context(), srcFile, dst, Into); err != nil {
		t.Fatal(err)
	}

	assertFileContent(t, filepath.Join(dst, "data.txt"), testData)
}

func TestCopyTreeInto(t *testing.T) {
	src := t.TempDir()
	createTree(t, src)

	dst := t.TempDir()

	if err := Copy(t.Context(), src, dst, Into); err != nil {
		t.Fatal(err)
	}

	base := filepath.Base(src)
	assertExists(t, filepath.Join(dst, base, "file1"))
	assertExists(t, filepath.Join(dst, base, "folder1", "folder2"))
	assertFileContent(t, filepath.Join(dst, base, "file1"), testData)
}

func TestCopyTreeIntoWithForce(t *testing.T) {
	src := t.TempDir()
	createTree(t, src)

	dst := t.TempDir()

	// First copy with Into.
	if err := Copy(t.Context(), src, dst, Into); err != nil {
		t.Fatal(err)
	}

	// Second copy with Into + Force to overwrite.
	if err := Copy(t.Context(), src, dst, Into, WithForce); err != nil {
		t.Fatal(err)
	}

	base := filepath.Base(src)
	assertFileContent(t, filepath.Join(dst, base, "file1"), testData)
}
