[![GoDoc](https://godoc.org/github.com/KarpelesLab/reflink?status.svg)](https://godoc.org/github.com/KarpelesLab/reflink)

# reflink

Perform reflink operation on compatible filesystems (btrfs or xfs).

## What is a reflink?

There are a number of type of links existing on Linux:

* symlinks
* hardlinks
* reflinks

Reflinks are a new kind of links found in btrfs and xfs which act similar to hard links, except modifying one of the two files will not change the other, and typically only the changed data will take space on the disk (copy-on-write).

## Can I use reflinks?

A machine needs to have a compatible OS and filesystem to perform reflinks. Known to work are:

* btrfs on Linux
* xfs on Linux

Other OSes have similar features, to be implemented in the future.

* Windows has `DUPLICATE_EXTENTS_TO_FILE`
* Solaris has `reflink`
* MacOS has `clonefile`

## Usage

```golang
err := reflink.Always("original_file.bin", "snapshot-001.bin")

// or

err := reflink.Auto("source_img.png", "modified_img.png")
```

`reflink.Always` will fail if reflink is not supported or didn't work for any reason, while `reflink.Auto` will fallback to a regular file copy.

# Notes

* The arguments have been put in the same order as `os.Link` or `os.Rename` rather than `io.Copy` as we are dealing with filenames.
