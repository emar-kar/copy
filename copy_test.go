package copy

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"strings"
	"testing"

	"github.com/emar-kar/copy/v2/internal/utils"
)

var (
	testData = []byte("test file data")

	folders = []string{
		"folder1", "folder1/folder2", "folder1/folder2_1", "folder1/folder2/folder3",
	}
	files    = []string{"file1", "folder1/folder2/file2", "folder1/folder2/folder3/file3"}
	symlinks = []string{"folder1/folder2:folder2", "folder1/folder2/folder3/file3:file3"}
)

type testCase struct {
	name     string
	src, dst string
	options  []optFunc
	context  func() context.Context
	preFunc  func() error
	resFunc  func(error) error
	postFunc func() error
}

func createSourceTree() (string, error) {
	temp, err := os.MkdirTemp("", "")
	if err != nil {
		return "", err
	}

	for _, f := range folders {
		if err := os.MkdirAll(path.Join(temp, f), 0o755); err != nil {
			return "", err
		}
	}

	for _, f := range files {
		p := path.Join(temp, f)
		if err := os.WriteFile(p, testData, 0o755); err != nil {
			return "", err
		}
	}

	for _, s := range symlinks {
		sp := strings.Split(s, ":")
		src, dst := path.Join(temp, sp[0]), path.Join(temp, sp[1])
		if err := os.Symlink(src, dst); err != nil {
			return "", err
		}
	}

	return temp, nil
}

func TestCopy(t *testing.T) {
	src, err := createSourceTree()
	if err != nil {
		t.Fatal(err)
	}

	dst := path.Join(os.TempDir(), "destination")

	defer func() {
		os.RemoveAll(src)
		os.RemoveAll(dst)
	}()

	tests := []testCase{
		{
			"FileCopy",
			path.Join(src, "file1"),
			dst + "/",
			[]optFunc{WithBufferSize(-1), Force},
			func() context.Context { return t.Context() },
			func() error {
				if err := os.MkdirAll(dst, 0o755); err != nil {
					return err
				}

				return os.WriteFile(
					path.Join(dst, "file1"), []byte("wrong data"), 0o755,
				)
			},
			func(_ error) error {
				p := path.Join(dst, "file1")
				if _, err := os.Stat(p); errors.Is(err, fs.ErrNotExist) {
					return err
				}

				data, err := os.ReadFile(p)
				if err != nil {
					return err
				}

				if !bytes.Equal(testData, data) {
					return fmt.Errorf(
						"byte slices are not equal: want: %v; got: %v", testData, data,
					)
				}

				return nil
			},
			func() error { return nil },
		},
		{
			"TreeCopy",
			src,
			dst,
			[]optFunc{WithBufferSize(1024)},
			func() context.Context { return t.Context() },
			func() error { return nil },
			func(cErr error) error {
				if cErr != nil {
					return cErr
				}

				for _, f := range append(folders, files...) {
					if _, err := os.Stat(path.Join(dst, f)); errors.Is(err, fs.ErrNotExist) {
						return err
					}
				}

				for _, s := range symlinks {
					sp := strings.Split(s, ":")
					_, link, err := utils.ResolvePath(path.Join(dst, sp[1]))
					if err != nil {
						return err
					}

					if link {
						return fmt.Errorf("%s: is a symlink", path.Join(dst, s))
					}
				}

				return nil
			},
			func() error { return nil },
		},
		{
			"FileCopySymlink",
			path.Join(src, "file3"),
			dst + "/",
			[]optFunc{WithNoFollow},
			func() context.Context { return t.Context() },
			func() error { return nil },
			func(_ error) error {
				_, link, err := utils.ResolvePath(path.Join(dst, "file3"))
				if err != nil {
					return err
				}

				if !link {
					return fmt.Errorf("%s: is not a symlink", path.Join(dst, "file3"))
				}

				return nil
			},
			func() error { return nil },
		},
		{
			"FileNoCopyExclude",
			path.Join(src, "file1"),
			dst + "/",
			[]optFunc{WithExcludeFunc(func(s string) bool { return path.Base(s) == "file1" })},
			func() context.Context { return t.Context() },
			func() error { return nil },
			func(cErr error) error {
				if cErr != nil {
					return cErr
				}

				if _, err := os.Stat(
					path.Join(dst, "file1"),
				); !errors.Is(err, fs.ErrNotExist) {
					return fmt.Errorf("%s: %w", path.Join(dst, "file1"), fs.ErrExist)
				}

				return nil
			},
			func() error { return nil },
		},
		{
			"CopyTreeNoFollow",
			src,
			dst,
			[]optFunc{
				WithNoFollow,
				WithExcludeFunc(func(s string) bool {
					return strings.Contains(s, "folder3")
				}),
			},
			func() context.Context { return t.Context() },
			func() error { return nil },
			func(cErr error) error {
				if cErr != nil {
					return cErr
				}

				for _, f := range append(folders, files...) {
					if strings.Contains(f, "folder3") {
						continue
					}

					if _, err := os.Stat(path.Join(dst, f)); errors.Is(err, fs.ErrNotExist) {
						return err
					}
				}

				for _, s := range symlinks {
					sp := strings.Split(s, ":")

					if strings.Contains(sp[0], "folder3") {
						continue
					}

					res, link, err := utils.ResolvePath(path.Join(dst, sp[1]))
					if err != nil {
						return err
					}

					if !link {
						return fmt.Errorf("%s: is not a symlink", path.Join(dst, sp[1]))
					}

					p, _, err := utils.ResolvePath(path.Join(src, sp[1]))
					if err != nil {
						return err
					}

					if res != p {
						return fmt.Errorf(
							"%s: wrong symlink: want: %s; got: %s",
							path.Join(dst, sp[1]), p, res,
						)
					}
				}

				return nil
			},
			func() error { return nil },
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.preFunc(); err != nil {
				t.Errorf("%s: preFunc error: %s", tc.name, err)
			}

			if err := tc.resFunc(Copy(
				tc.context(),
				tc.src,
				tc.dst,
				tc.options...,
			)); err != nil {
				t.Errorf("%s: error: %s", tc.name, err)
			}

			if err := tc.postFunc(); err != nil {
				t.Errorf("%s: postFunc error: %s", tc.name, err)
			}

			if err := os.RemoveAll(dst); err != nil {
				t.Errorf("%s: cleanup error: %s", tc.name, err)
			}
		})
	}
}

func TestCopySameFile(t *testing.T) {
	temp, err := os.CreateTemp("", "")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(temp.Name())

	if err := Copy(t.Context(), temp.Name(), temp.Name()); err == nil {
		t.Fatal("error was expected, but copy was successful")
	} else if !errors.Is(err, ErrSame) {
		t.Fatalf("unexpected error: want %s; got: %s", ErrSame, err)
	}
}

func TestCopyAlreadyExists(t *testing.T) {
	src, err := os.CreateTemp("", "")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(src.Name())

	dst, err := os.CreateTemp("", "")
	if err != nil {
		os.RemoveAll(src.Name())
		t.Fatal(err)
	}
	defer os.RemoveAll(dst.Name())

	if err := Copy(t.Context(), src.Name(), dst.Name()); err == nil {
		t.Fatal("error was expected, but copy was successful")
	} else if !errors.Is(err, fs.ErrExist) {
		t.Fatalf("unexpected error: want %s; got: %s", fs.ErrExist, err)
	}
}
