package main

import (
	"errors"
	"fmt"
	"os"
	"syscall"

	"bazil.org/fuse"
	"bazil.org/fuse/fs"
	"github.com/boltdb/bolt"
)

type File struct {
	dir  *Dir
	size uint64
	name []byte
}

var _ = fs.Node(&File{})

func (f *File) Attr() fuse.Attr {
	return fuse.Attr{Mode: 0644, Size: f.size}
}

var _ = fs.NodeOpener(&File{})

func (f *File) Open(req *fuse.OpenRequest, resp *fuse.OpenResponse, intr fs.Intr) (fs.Handle, fuse.Error) {
	var h fs.Handle
	switch int(req.Flags & syscall.O_ACCMODE) {
	case os.O_RDONLY:
		err := f.dir.root.fs.db.View(func(tx *bolt.Tx) error {
			b := f.dir.bucket(tx)
			if b == nil {
				return errors.New("bucket no longer exists")
			}
			v := b.Get(f.name)
			if v == nil {
				return fuse.ESTALE
			}
			h = fs.DataHandle(v)
			return nil
		})
		if err != nil {
			return nil, err
		}
	case os.O_RDWR, os.O_WRONLY:
		return nil, fuse.EPERM
	default:
		return nil, fmt.Errorf("weird open request flags: %v", req.Flags)
	}
	return h, nil
}
