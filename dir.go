package main

import (
	"errors"
	"os"

	"bazil.org/fuse"
	"bazil.org/fuse/fs"
	"github.com/boltdb/bolt"
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
				Name: string(k),
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
		nameB := []byte(name)
		if child := b.Bucket(nameB); child != nil {
			// directory
			buckets := make([][]byte, 0, len(d.buckets)+1)
			buckets = append(buckets, d.buckets...)
			buckets = append(buckets, nameB)
			n = &Dir{
				root:    d.root,
				buckets: buckets,
			}
			return nil
		}
		if child := b.Get(nameB); child != nil {
			// file
			n = &File{
				dir:  d,
				size: uint64(len(child)),
				name: nameB,
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
