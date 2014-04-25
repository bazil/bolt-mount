package main

import (
	"os"
	"syscall"

	"github.com/boltdb/bolt"

	"bazil.org/fuse"
	"bazil.org/fuse/fs"
)

// Root is a Bolt transaction root bucket. It's special because it
// cannot contain keys, and doesn't really have a *bolt.Bucket.
type Root struct {
	fs *FS
}

var _ = fs.Node(&Root{})

func (r *Root) Attr() fuse.Attr {
	return fuse.Attr{Inode: 1, Mode: os.ModeDir | 0755}
}

var _ = fs.HandleReadDirer(&Root{})

func (r *Root) ReadDir(intr fs.Intr) ([]fuse.Dirent, fuse.Error) {
	var res []fuse.Dirent
	err := r.fs.db.View(func(tx *bolt.Tx) error {
		return tx.ForEach(func(name []byte, b *bolt.Bucket) error {
			res = append(res, fuse.Dirent{
				Type: fuse.DT_Dir,
				Name: string(name),
			})
			return nil
		})
	})
	return res, err
}

var _ = fs.NodeStringLookuper(&Root{})

func (r *Root) Lookup(name string, intr fs.Intr) (fs.Node, fuse.Error) {
	var n fs.Node
	err := r.fs.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(name))
		if b == nil {
			return fuse.ENOENT
		}
		n = &Dir{
			root:    r,
			buckets: [][]byte{[]byte(name)},
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return n, nil
}

var _ = fs.NodeMkdirer(&Root{})

func (r *Root) Mkdir(req *fuse.MkdirRequest, intr fs.Intr) (fs.Node, fuse.Error) {
	name := []byte(req.Name)
	err := r.fs.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(name)
		if b != nil {
			return fuse.Errno(syscall.EEXIST)
		}
		if _, err := tx.CreateBucket(name); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	n := &Dir{
		root:    r,
		buckets: [][]byte{name},
	}
	return n, nil
}
