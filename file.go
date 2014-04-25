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
	var data []byte

	err := f.dir.root.fs.db.View(func(tx *bolt.Tx) error {
		b := f.dir.bucket(tx)
		if b == nil {
			return errors.New("bucket no longer exists")
		}
		v := b.Get(f.name)
		if v == nil {
			return fuse.ESTALE
		}
		// make a copy because once we're out of the transaction,
		// bolt might reuse the db page
		data = append([]byte(nil), v...)
		return nil
	})
	if err != nil {
		return nil, err
	}

	switch int(req.Flags & syscall.O_ACCMODE) {
	case os.O_RDONLY:
		h = fs.DataHandle(data)
	case os.O_RDWR, os.O_WRONLY:
		h = &FileHandle{
			file: f,
			data: data,
		}
	default:
		return nil, fmt.Errorf("weird open request flags: %v", req.Flags)
	}
	return h, nil
}

type FileHandle struct {
	file *File
	data []byte
}

var _ = fs.Handle(&FileHandle{})

var _ = fs.HandleWriter(&FileHandle{})

const maxInt = int(^uint(0) >> 1)

func (h *FileHandle) Write(req *fuse.WriteRequest, resp *fuse.WriteResponse, intr fs.Intr) fuse.Error {
	// expand the buffer if necessary
	newLen := req.Offset + int64(len(req.Data))
	if newLen > int64(maxInt) {
		return fuse.Errno(syscall.EFBIG)
	}

	n := copy(h.data[req.Offset:], req.Data)
	if n < len(req.Data) {
		h.data = append(h.data, req.Data[n:]...)
	}
	resp.Size = len(req.Data)
	return nil
}

var _ = fs.HandleFlusher(&FileHandle{})

func (h *FileHandle) Flush(req *fuse.FlushRequest, intr fs.Intr) fuse.Error {
	err := h.file.dir.root.fs.db.Update(func(tx *bolt.Tx) error {
		b := h.file.dir.bucket(tx)
		if b == nil {
			return fuse.ESTALE
		}
		return b.Put(h.file.name, h.data)
	})
	if err != nil {
		return err
	}
	return nil
}
