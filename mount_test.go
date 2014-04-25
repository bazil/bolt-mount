package main

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"bazil.org/fuse/fs/fstestutil"
	"github.com/boltdb/bolt"
)

func withDB(t testing.TB, fn func(*bolt.DB)) {
	tmp, err := ioutil.TempFile("", "bolt-mount-test-")
	if err != nil {
		t.Fatal(err)
	}
	dbpath := tmp.Name()
	defer func() {
		_ = os.Remove(dbpath)
	}()
	_ = tmp.Close()
	db, err := bolt.Open(dbpath, 0600, nil)
	if err != nil {
		t.Fatal(err)
	}
	fn(db)
}

func withMount(t testing.TB, db *bolt.DB, fn func(mntpath string)) {
	filesys := &FS{
		db: db,
	}
	mnt, err := fstestutil.MountedT(t, filesys)
	if err != nil {
		t.Fatal(err)
	}
	defer mnt.Close()
	fn(mnt.Dir)
}

type fileInfo struct {
	name string
	size int64
	mode os.FileMode
}

func checkFI(t testing.TB, got os.FileInfo, expected fileInfo) {
	if g, e := got.Name(), expected.name; g != e {
		t.Errorf("file info has bad name: %q != %q", g, e)
	}
	if g, e := got.Size(), expected.size; g != e {
		t.Errorf("file info has bad size: %v != %v", g, e)
	}
	if g, e := got.Mode(), expected.mode; g != e {
		t.Errorf("file info has bad mode: %v != %v", g, e)
	}
}

func TestRootReaddir(t *testing.T) {
	withDB(t, func(db *bolt.DB) {
		err := db.Update(func(tx *bolt.Tx) error {
			if _, err := tx.CreateBucket([]byte("one")); err != nil {
				return err
			}
			if _, err := tx.CreateBucket([]byte("two")); err != nil {
				return err
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		withMount(t, db, func(mntpath string) {
			fis, err := ioutil.ReadDir(mntpath)
			if err != nil {
				t.Fatal(err)
			}
			if g, e := len(fis), 2; g != e {
				t.Fatalf("wrong readdir results: got %v", fis)
			}
			checkFI(t, fis[0], fileInfo{name: "one", size: 0, mode: 0755 | os.ModeDir})
			checkFI(t, fis[1], fileInfo{name: "two", size: 0, mode: 0755 | os.ModeDir})
		})
	})
}

func TestRootMkdir(t *testing.T) {
	withDB(t, func(db *bolt.DB) {
		withMount(t, db, func(mntpath string) {
			if err := os.Mkdir(filepath.Join(mntpath, "one"), 0700); err != nil {
				t.Error(err)
			}
		})
		check := func(tx *bolt.Tx) error {
			b := tx.Bucket([]byte("one"))
			if b == nil {
				t.Error("expected to see bucket 'one'")
			}
			return nil
		}
		if err := db.View(check); err != nil {
			t.Fatal(err)
		}
	})
}
