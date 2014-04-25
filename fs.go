package main

import (
	"bazil.org/fuse"
	"bazil.org/fuse/fs"
	"github.com/boltdb/bolt"
)

type FS struct {
	db *bolt.DB
}

var _ = fs.FS(&FS{})

func (f *FS) Root() (fs.Node, fuse.Error) {
	n := &Root{
		fs: f,
	}
	return n, nil
}
