# copy

Simple copy module for Go applications, supporting files, directory trees, symlinks, context cancellation, and configurable options.

## Installation

```bash
go get github.com/emar-kar/copy/v2
```

## Usage

```go
// Copy a file.
err := copy.Copy(ctx, "src/file.txt", "dst/file.txt")

// Copy a directory tree.
err := copy.Copy(ctx, "src/dir", "dst/dir")

// Copy into a directory (preserves source name).
err := copy.Copy(ctx, "src/file.txt", "dst/dir/")
// Or equivalently:
err := copy.Copy(ctx, "src/file.txt", "dst/dir", copy.Into)
```

## Behaviour

| Source | Destination | Result |
|--------|-------------|--------|
| File | New path | File created at that path |
| File | Trailing `/` or `Into` | File created inside the directory under its original name |
| Directory | New path | Full tree copied recursively |
| Directory | Trailing `/` or `Into` | Full tree copied recursively with root folder |
| Symlink | Any | Target resolved and copied (file or tree); use `WithNoFollow` to preserve the symlink |

### Errors

| Error | Condition |
|-------|-----------|
| `ErrSame` | Source and destination resolve to the same inode |
| `fs.ErrExist` | Destination already exists and `WithForce` is not set |

## Options

Options are passed as trailing variadic arguments to `Copy`.

### `WithForce`

Removes the destination before copying if it already exists.

```go
err := copy.Copy(ctx, src, dst, copy.WithForce)
```

> `Force` is a deprecated alias for `WithForce`.

### `WithNoFollow`

Recreates symlinks at the destination instead of following them to their targets. Broken symlinks are preserved as-is.

```go
err := copy.Copy(ctx, src, dst, copy.WithNoFollow)
```

### `WithBufferSize`

Sets the size of the in-memory buffer used when copying file contents. If `size <= 0` the default of 64 KiB is used.

```go
err := copy.Copy(ctx, src, dst, copy.WithBufferSize(256*1024))
```

### `WithExcludeFunc`

Sets a predicate that receives the **absolute path** of each file or directory encountered. Returning `true` skips that entry; for directories the entire subtree is skipped.

```go
err := copy.Copy(ctx, src, dst, copy.WithExcludeFunc(func(path string) bool {
    return filepath.Base(path) == ".git"
}))
```

### `Into`

Copies src inside dst under its base name. This is the programmatic equivalent of adding a trailing `/` to the destination path.

```go
// These two are equivalent:
err := copy.Copy(ctx, "src/dir", "dst/target/")          // trailing slash
err := copy.Copy(ctx, "src/dir", "dst/target", copy.Into) // Into option
// result: dst/target/dir/{files...}
```

## Context cancellation

`Copy` respects the provided context and returns the context error as soon as it is cancelled, stopping any ongoing copy.

```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

err := copy.Copy(ctx, src, dst)
```

## Combining options

All options can be combined freely:

```go
err := copy.Copy(ctx, src, dst,
    copy.Into,
    copy.WithForce,
    copy.WithNoFollow,
    copy.WithBufferSize(128*1024),
    copy.WithExcludeFunc(func(path string) bool {
        return strings.HasSuffix(path, ".tmp")
    }),
)
```
