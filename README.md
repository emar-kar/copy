# copy
Simple copy module for go applications, which allows to create copies of files and folders.

## Available options

- `Force`: re-writes destination if it is already exists.
- `ContentOnly`: copies only source folder content without creating root folder in destination.
- `WithMove`: removes source after copying process is finished.
- `WithBufferSize`: allows to set custom buffer size for file copy. If provided size <= 0, then default 4096 will be used.
- `RevertOnErr`: removes destination file if there was an error during copy process.
- `WithHash`: calculates hash of the copied files for client's use.

### Paths processing
If you need to copy file with the same name to destination, you need to add trailing slash at the destination path, since it will consider the last path element as the file name. The file name might be the same as the source file or a new custom one.
