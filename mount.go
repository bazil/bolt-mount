package main

import (
	"bazil.org/fuse"
	"bazil.org/fuse/fs"
	"github.com/boltdb/bolt"
)

func mount(dbpath, mountpoint string) error {
	db, err := bolt.Open(dbpath, 0600, nil)
	if err != nil {
		return err
	}

	c, err := fuse.Mount(mountpoint)
	if err != nil {
		return err
	}
	defer c.Close()

	filesys := &FS{
		db: db,
	}
	if err := fs.Serve(c, filesys); err != nil {
		return err
	}

	// check if the mount process has an error to report
	<-c.Ready
	if err := c.MountError; err != nil {
		return err
	}

	return nil
}
