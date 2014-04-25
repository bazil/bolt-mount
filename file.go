package main

import (
	"bazil.org/fuse"
	"bazil.org/fuse/fs"
)

type File struct {
	dir  *Dir
	name []byte
}

var _ = fs.Node(&File{})

func (f *File) Attr() fuse.Attr {
	return fuse.Attr{Mode: 0644}
}
