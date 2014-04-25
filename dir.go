package main

import (
	"os"

	"bazil.org/fuse"
	"bazil.org/fuse/fs"
)

type Dir struct {
	root *Root
}

var _ = fs.Node(&Dir{})

func (d *Dir) Attr() fuse.Attr {
	return fuse.Attr{Mode: os.ModeDir | 0755}
}
