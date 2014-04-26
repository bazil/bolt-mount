package main

import (
	"errors"
	"os"
	"syscall"

	"github.com/boltdb/bolt"

	"bazil.org/fuse"
	"bazil.org/fuse/fs"
)

type Dir struct {
	root    *Root
	buckets [][]byte
}

var _ = fs.Node(&Dir{})

func (d *Dir) Attr() fuse.Attr {
	return fuse.Attr{Mode: os.ModeDir | 0755}
}

var _ = fs.HandleReadDirer(&Root{})

func (d *Dir) bucket(tx *bolt.Tx) *bolt.Bucket {
	b := tx.Bucket(d.buckets[0])
	if b == nil {
		return nil
	}
	for _, name := range d.buckets[1:] {
		b = b.Bucket(name)
		if b == nil {
			return nil
		}
	}
	return b
}

func (d *Dir) ReadDir(intr fs.Intr) ([]fuse.Dirent, fuse.Error) {
	var res []fuse.Dirent
	err := d.root.fs.db.View(func(tx *bolt.Tx) error {
		b := d.bucket(tx)
		if b == nil {
			return errors.New("bucket no longer exists")
		}
		c := b.Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			de := fuse.Dirent{
				Name: EncodeKey(k),
			}
			if v == nil {
				de.Type = fuse.DT_Dir
			} else {
				de.Type = fuse.DT_File
			}
			res = append(res, de)
		}
		return nil
	})
	return res, err
}

var _ = fs.NodeStringLookuper(&Dir{})

func (d *Dir) Lookup(name string, intr fs.Intr) (fs.Node, fuse.Error) {
	var n fs.Node
	err := d.root.fs.db.View(func(tx *bolt.Tx) error {
		b := d.bucket(tx)
		if b == nil {
			return errors.New("bucket no longer exists")
		}
		nameRaw, err := DecodeKey(name)
		if err != nil {
			return fuse.ENOENT
		}
		if child := b.Bucket(nameRaw); child != nil {
			// directory
			buckets := make([][]byte, 0, len(d.buckets)+1)
			buckets = append(buckets, d.buckets...)
			buckets = append(buckets, nameRaw)
			n = &Dir{
				root:    d.root,
				buckets: buckets,
			}
			return nil
		}
		if child := b.Get(nameRaw); child != nil {
			// file
			n = &File{
				dir:  d,
				name: nameRaw,
			}
			return nil
		}
		return fuse.ENOENT
	})
	if err != nil {
		return nil, err
	}
	return n, nil
}

var _ = fs.NodeMkdirer(&Dir{})

func (d *Dir) Mkdir(req *fuse.MkdirRequest, intr fs.Intr) (fs.Node, fuse.Error) {
	name, err := DecodeKey(req.Name)
	if err != nil {
		return nil, fuse.EPERM
	}
	err = d.root.fs.db.Update(func(tx *bolt.Tx) error {
		b := d.bucket(tx)
		if b == nil {
			return errors.New("bucket no longer exists")
		}
		if child := b.Bucket(name); child != nil {
			return fuse.Errno(syscall.EEXIST)
		}
		if _, err := b.CreateBucket(name); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	buckets := make([][]byte, 0, len(d.buckets)+1)
	buckets = append(buckets, d.buckets...)
	buckets = append(buckets, name)
	n := &Dir{
		root:    d.root,
		buckets: buckets,
	}
	return n, nil
}

var _ = fs.NodeCreater(&Dir{})

func (d *Dir) Create(req *fuse.CreateRequest, resp *fuse.CreateResponse, intr fs.Intr) (fs.Node, fs.Handle, fuse.Error) {
	nameRaw, err := DecodeKey(req.Name)
	if err != nil {
		return nil, nil, fuse.EPERM
	}
	f := &File{
		dir:  d,
		name: nameRaw,
	}
	h := &FileHandle{
		file: f,
		data: nil,
	}
	return f, h, nil
}
