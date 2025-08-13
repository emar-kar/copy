# copy
Simple copy module for go applications, which allows to create copies of files and folders.

## Installation
```bash
go get github.com/emar-kar/copy/v2
```

## Available options

- `WithBufferSize`: allows to set custom buffer size for file copy. If provided size <= 0, then default 65536 bytes will be used.
- `WithExcludeFunc`: allows to set custom function to exclude paths.
- `Force`: re-writes destination if it already exists.
- `WithNoFollow`: creates symlinks instead of resolving paths.

### Paths processing
If you need to copy file or folder with the same name as source in destination, just add trailing slash at the destination path, since it will consider the last path element as the file name.
