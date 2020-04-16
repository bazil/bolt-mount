package main

import (
	"context"
	"errors"
	"os"

	"bazil.org/fuse"
	"bazil.org/fuse/fs"
	bolt "go.etcd.io/bbolt"
)

type Dir struct {
	fs *FS
	// path from Bolt database root to this bucket; empty for root bucket
	buckets [][]byte
}

var _ = fs.Node(&Dir{})

func (d *Dir) Attr(ctx context.Context, a *fuse.Attr) error {
	a.Mode = os.ModeDir | 0755
	return nil
}

var _ = fs.HandleReadDirAller(&Dir{})

type BucketLike interface {
	Bucket(name []byte) *bolt.Bucket
	CreateBucket(name []byte) (*bolt.Bucket, error)
	DeleteBucket(name []byte) error
	Cursor() *bolt.Cursor
	Get(key []byte) []byte
	Put(key []byte, value []byte) error
	Delete(key []byte) error
}

// root bucket is special because it cannot contain keys, and doesn't
// really have a *bolt.Bucket exposed in the Bolt API.
type fakeBucket struct {
	*bolt.Tx
}

func (fakeBucket) Get(key []byte) []byte {
	return nil
}

func (fakeBucket) Put(key []byte, value []byte) error {
	return fuse.EPERM
}

func (fakeBucket) Delete(key []byte) error {
	return fuse.EPERM
}

// bucket returns a BucketLike value (either a *bolt.Bucket or a
// *bolt.Tx wrapped in a fakeBucket, to provide Get/Put/Delete
// methods).
//
// It never returns a nil value in a non-nil interface.
func (d *Dir) bucket(tx *bolt.Tx) BucketLike {
	if len(d.buckets) == 0 {
		return fakeBucket{tx}
	}
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

func (d *Dir) ReadDirAll(ctx context.Context) ([]fuse.Dirent, error) {
	var res []fuse.Dirent
	err := d.fs.db.View(func(tx *bolt.Tx) error {
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

func (d *Dir) Lookup(ctx context.Context, name string) (fs.Node, error) {
	var n fs.Node
	err := d.fs.db.View(func(tx *bolt.Tx) error {
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
				fs:      d.fs,
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

func (d *Dir) Mkdir(ctx context.Context, req *fuse.MkdirRequest) (fs.Node, error) {
	name, err := DecodeKey(req.Name)
	if err != nil {
		return nil, fuse.EPERM
	}
	err = d.fs.db.Update(func(tx *bolt.Tx) error {
		b := d.bucket(tx)
		if b == nil {
			return errors.New("bucket no longer exists")
		}
		if child := b.Bucket(name); child != nil {
			return fuse.EEXIST
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
		fs:      d.fs,
		buckets: buckets,
	}
	return n, nil
}

var _ = fs.NodeCreater(&Dir{})

func (d *Dir) Create(ctx context.Context, req *fuse.CreateRequest, resp *fuse.CreateResponse) (fs.Node, fs.Handle, error) {
	if len(d.buckets) == 0 {
		// only buckets go in root bucket
		return nil, nil, fuse.EPERM
	}
	nameRaw, err := DecodeKey(req.Name)
	if err != nil {
		return nil, nil, fuse.EPERM
	}
	f := &File{
		dir:     d,
		name:    nameRaw,
		writers: 1,
		// file is empty at Create time, no need to set data
	}
	return f, f, nil
}

var _ = fs.NodeRemover(&Dir{})

func (d *Dir) Remove(ctx context.Context, req *fuse.RemoveRequest) error {
	nameRaw, err := DecodeKey(req.Name)
	if err != nil {
		return fuse.ENOENT
	}
	fn := func(tx *bolt.Tx) error {
		b := d.bucket(tx)
		if b == nil {
			return errors.New("bucket no longer exists")
		}

		switch req.Dir {
		case true:
			if b.Bucket(nameRaw) == nil {
				return fuse.ENOENT
			}
			if err := b.DeleteBucket(nameRaw); err != nil {
				return err
			}

		case false:
			if b.Get(nameRaw) == nil {
				return fuse.ENOENT
			}
			if err := b.Delete(nameRaw); err != nil {
				return err
			}
		}
		return nil
	}
	return d.fs.db.Update(fn)
}
