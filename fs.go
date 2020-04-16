package main

import (
	"bazil.org/fuse/fs"
	bolt "go.etcd.io/bbolt"
)

type FS struct {
	db *bolt.DB
}

var _ = fs.FS(&FS{})

func (f *FS) Root() (fs.Node, error) {
	n := &Dir{
		fs: f,
	}
	return n, nil
}
