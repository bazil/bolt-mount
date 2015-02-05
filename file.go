package main

import (
	"errors"
	"sync"
	"syscall"

	"bazil.org/fuse"
	"bazil.org/fuse/fs"
	"bazil.org/fuse/fuseutil"
	"github.com/boltdb/bolt"
	"golang.org/x/net/context"
)

type File struct {
	dir  *Dir
	name []byte

	mu sync.Mutex
	// number of write-capable handles currently open
	writers uint
	// only valid if writers > 0
	data []byte
}

var _ = fs.Node(&File{})
var _ = fs.Handle(&File{})

// load calls fn inside a View with the contents of the file. Caller
// must make a copy of the data if needed, because once we're out of
// the transaction, bolt might reuse the db page.
func (f *File) load(fn func([]byte)) error {
	err := f.dir.fs.db.View(func(tx *bolt.Tx) error {
		b := f.dir.bucket(tx)
		if b == nil {
			return errors.New("bucket no longer exists")
		}
		v := b.Get(f.name)
		if v == nil {
			return fuse.ESTALE
		}
		fn(v)
		return nil
	})
	return err
}

func (f *File) Attr() fuse.Attr {
	f.mu.Lock()
	defer f.mu.Unlock()

	attr := fuse.Attr{Mode: 0644, Size: uint64(len(f.data))}
	if f.writers == 0 {
		// not in memory, fetch correct size.
		// Attr can't fail, so ignore errors
		_ = f.load(func(b []byte) { attr.Size = uint64(len(b)) })
	}
	return attr
}

var _ = fs.NodeOpener(&File{})

func (f *File) Open(ctx context.Context, req *fuse.OpenRequest, resp *fuse.OpenResponse) (fs.Handle, fuse.Error) {
	if req.Flags.IsReadOnly() {
		// we don't need to track read-only handles
		return f, nil
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	if f.writers == 0 {
		// load data
		fn := func(b []byte) {
			f.data = append([]byte(nil), b...)
		}
		if err := f.load(fn); err != nil {
			return nil, err
		}
	}

	f.writers++
	return f, nil
}

var _ = fs.HandleReleaser(&File{})

func (f *File) Release(ctx context.Context, req *fuse.ReleaseRequest) fuse.Error {
	if req.Flags.IsReadOnly() {
		// we don't need to track read-only handles
		return nil
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	f.writers--
	if f.writers == 0 {
		f.data = nil
	}
	return nil
}

var _ = fs.HandleReader(&File{})

func (f *File) Read(ctx context.Context, req *fuse.ReadRequest, resp *fuse.ReadResponse) fuse.Error {
	f.mu.Lock()
	defer f.mu.Unlock()

	fn := func(b []byte) {
		fuseutil.HandleRead(req, resp, b)
	}
	if f.writers == 0 {
		f.load(fn)
	} else {
		fn(f.data)
	}
	return nil
}

var _ = fs.HandleWriter(&File{})

const maxInt = int(^uint(0) >> 1)

func (f *File) Write(ctx context.Context, req *fuse.WriteRequest, resp *fuse.WriteResponse) fuse.Error {
	f.mu.Lock()
	defer f.mu.Unlock()

	// expand the buffer if necessary
	newLen := req.Offset + int64(len(req.Data))
	if newLen > int64(maxInt) {
		return fuse.Errno(syscall.EFBIG)
	}

	n := copy(f.data[req.Offset:], req.Data)
	if n < len(req.Data) {
		f.data = append(f.data, req.Data[n:]...)
	}
	resp.Size = len(req.Data)
	return nil
}

var _ = fs.HandleFlusher(&File{})

func (f *File) Flush(ctx context.Context, req *fuse.FlushRequest) fuse.Error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.writers == 0 {
		// Read-only handles also get flushes. Make sure we don't
		// overwrite valid file contents with a nil buffer.
		return nil
	}

	err := f.dir.fs.db.Update(func(tx *bolt.Tx) error {
		b := f.dir.bucket(tx)
		if b == nil {
			return fuse.ESTALE
		}
		return b.Put(f.name, f.data)
	})
	if err != nil {
		return err
	}
	return nil
}
