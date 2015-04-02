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

func TestRootRmdir(t *testing.T) {
	withDB(t, func(db *bolt.DB) {
		prep := func(tx *bolt.Tx) error {
			if _, err := tx.CreateBucket([]byte("one")); err != nil {
				return err
			}
			return nil
		}
		if err := db.Update(prep); err != nil {
			t.Fatal(err)
		}

		withMount(t, db, func(mntpath string) {
			if err := os.Remove(filepath.Join(mntpath, "one")); err != nil {
				t.Error(err)
			}
		})
		check := func(tx *bolt.Tx) error {
			b := tx.Bucket([]byte("one"))
			if b != nil {
				t.Error("bucket 'one' is still there")
			}
			return nil
		}
		if err := db.View(check); err != nil {
			t.Fatal(err)
		}
	})
}

func TestBucketReaddir(t *testing.T) {
	withDB(t, func(db *bolt.DB) {
		prep := func(tx *bolt.Tx) error {
			b, err := tx.CreateBucket([]byte("bukkit"))
			if err != nil {
				return err
			}
			if _, err := b.CreateBucket([]byte("one")); err != nil {
				return err
			}
			if err := b.Put([]byte("two"), []byte("hello")); err != nil {
				return err
			}
			return nil
		}
		if err := db.Update(prep); err != nil {
			t.Fatal(err)
		}
		withMount(t, db, func(mntpath string) {
			fis, err := ioutil.ReadDir(filepath.Join(mntpath, "bukkit"))
			if err != nil {
				t.Fatal(err)
			}
			if g, e := len(fis), 2; g != e {
				t.Fatalf("wrong readdir results: got %v", fis)
			}
			checkFI(t, fis[0], fileInfo{name: "one", size: 0, mode: 0755 | os.ModeDir})
			checkFI(t, fis[1], fileInfo{name: "two", size: 5, mode: 0644})
		})
	})
}

func TestBucketMkdir(t *testing.T) {
	withDB(t, func(db *bolt.DB) {
		prep := func(tx *bolt.Tx) error {
			_, err := tx.CreateBucket([]byte("bukkit"))
			if err != nil {
				return err
			}
			return nil
		}
		if err := db.Update(prep); err != nil {
			t.Fatal(err)
		}
		withMount(t, db, func(mntpath string) {
			if err := os.Mkdir(filepath.Join(mntpath, "bukkit", "sub"), 0700); err != nil {
				t.Error(err)
			}
		})
		check := func(tx *bolt.Tx) error {
			b := tx.Bucket([]byte("bukkit"))
			if b == nil {
				t.Error("expected to see bucket 'bukkit'")
			}
			b = b.Bucket([]byte("sub"))
			if b == nil {
				t.Error("expected to see bucket 'bukkit' 'sub'")
			}
			return nil
		}
		if err := db.View(check); err != nil {
			t.Fatal(err)
		}
	})
}

func TestBucketRmdir(t *testing.T) {
	withDB(t, func(db *bolt.DB) {
		prep := func(tx *bolt.Tx) error {
			b, err := tx.CreateBucket([]byte("bukkit"))
			if err != nil {
				return err
			}
			if _, err := b.CreateBucket([]byte("sub")); err != nil {
				return err
			}
			return nil
		}
		if err := db.Update(prep); err != nil {
			t.Fatal(err)
		}
		withMount(t, db, func(mntpath string) {
			if err := os.Remove(filepath.Join(mntpath, "bukkit", "sub")); err != nil {
				t.Error(err)
			}
		})
		check := func(tx *bolt.Tx) error {
			b := tx.Bucket([]byte("bukkit"))
			if b == nil {
				t.Error("expected to see bucket 'bukkit'")
			}
			b = b.Bucket([]byte("sub"))
			if b != nil {
				t.Error("bucket 'one' is still there")
			}
			return nil
		}
		if err := db.View(check); err != nil {
			t.Fatal(err)
		}
	})
}

func TestRead(t *testing.T) {
	withDB(t, func(db *bolt.DB) {
		prep := func(tx *bolt.Tx) error {
			b, err := tx.CreateBucket([]byte("bukkit"))
			if err != nil {
				return err
			}
			if err := b.Put([]byte("greeting"), []byte("hello")); err != nil {
				return err
			}
			return nil
		}
		if err := db.Update(prep); err != nil {
			t.Fatal(err)
		}
		withMount(t, db, func(mntpath string) {
			data, err := ioutil.ReadFile(filepath.Join(mntpath, "bukkit", "greeting"))
			if err != nil {
				t.Fatal(err)
			}
			if g, e := string(data), "hello"; g != e {
				t.Fatalf("wrong read results: %q != %q", g, e)
			}
		})
	})
}

func TestWrite(t *testing.T) {
	withDB(t, func(db *bolt.DB) {
		prep := func(tx *bolt.Tx) error {
			_, err := tx.CreateBucket([]byte("bukkit"))
			if err != nil {
				return err
			}
			return nil
		}
		if err := db.Update(prep); err != nil {
			t.Fatal(err)
		}
		withMount(t, db, func(mntpath string) {
			if err := ioutil.WriteFile(
				filepath.Join(mntpath, "bukkit", "greeting"),
				[]byte("hello"),
				0600,
			); err != nil {
				t.Fatal(err)
			}
		})
		check := func(tx *bolt.Tx) error {
			b := tx.Bucket([]byte("bukkit"))
			if b == nil {
				t.Fatalf("bukkit disappeared")
			}
			v := b.Get([]byte("greeting"))
			if g, e := string(v), "hello"; g != e {
				t.Fatalf("wrong write content: %q != %q", g, e)
			}
			return nil
		}
		if err := db.View(check); err != nil {
			t.Fatal(err)
		}
	})
}

func TestReadNested(t *testing.T) {
	withDB(t, func(db *bolt.DB) {
		prep := func(tx *bolt.Tx) error {
			b, err := tx.CreateBucket([]byte("bukkit"))
			if err != nil {
				return err
			}
			b, err = b.CreateBucket([]byte("sub"))
			if err != nil {
				return err
			}
			if err := b.Put([]byte("greeting"), []byte("hello")); err != nil {
				return err
			}
			return nil
		}
		if err := db.Update(prep); err != nil {
			t.Fatal(err)
		}
		withMount(t, db, func(mntpath string) {
			data, err := ioutil.ReadFile(filepath.Join(mntpath, "bukkit", "sub", "greeting"))
			if err != nil {
				t.Fatal(err)
			}
			if g, e := string(data), "hello"; g != e {
				t.Fatalf("wrong read results: %q != %q", g, e)
			}
		})
	})
}

func TestWriteNested(t *testing.T) {
	withDB(t, func(db *bolt.DB) {
		withMount(t, db, func(mntpath string) {
			if err := os.Mkdir(filepath.Join(mntpath, "bukkit"), 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(filepath.Join(mntpath, "bukkit", "sub"), 0755); err != nil {
				t.Fatal(err)
			}
			if err := ioutil.WriteFile(
				filepath.Join(mntpath, "bukkit", "sub", "greeting"),
				[]byte("hello"),
				0600,
			); err != nil {
				t.Fatal(err)
			}
		})
		check := func(tx *bolt.Tx) error {
			b := tx.Bucket([]byte("bukkit"))
			if b == nil {
				t.Fatalf("bukkit disappeared")
			}
			b = b.Bucket([]byte("sub"))
			if b == nil {
				t.Fatalf("sub-bukkit disappeared")
			}
			v := b.Get([]byte("greeting"))
			if g, e := string(v), "hello"; g != e {
				t.Fatalf("wrong write content: %q != %q", g, e)
			}
			return nil
		}
		if err := db.View(check); err != nil {
			t.Fatal(err)
		}
	})
}

func TestRemove(t *testing.T) {
	withDB(t, func(db *bolt.DB) {
		prep := func(tx *bolt.Tx) error {
			b, err := tx.CreateBucket([]byte("bukkit"))
			if err != nil {
				return err
			}
			if err := b.Put([]byte("greeting"), []byte("hello")); err != nil {
				return err
			}
			return nil
		}
		if err := db.Update(prep); err != nil {
			t.Fatal(err)
		}
		withMount(t, db, func(mntpath string) {
			if err := os.Remove(filepath.Join(mntpath, "bukkit", "greeting")); err != nil {
				t.Fatal(err)
			}
		})
		check := func(tx *bolt.Tx) error {
			b := tx.Bucket([]byte("bukkit"))
			if b == nil {
				t.Fatalf("bukkit disappeared")
			}
			v := b.Get([]byte("greeting"))
			if v != nil {
				t.Errorf("greeting is still there: %q", v)
			}
			return nil
		}
		if err := db.View(check); err != nil {
			t.Fatal(err)
		}
	})
}

func TestBinary(t *testing.T) {
	withDB(t, func(db *bolt.DB) {
		err := db.Update(func(tx *bolt.Tx) error {
			b, err := tx.CreateBucket([]byte("evil\x00lol/mwahaha"))
			if err != nil {
				return err
			}
			if err := b.Put([]byte("\x01\x02foobar"), []byte("hello")); err != nil {
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
			if g, e := len(fis), 1; g != e {
				t.Fatalf("wrong readdir results: got %v", fis)
			}
			checkFI(t, fis[0], fileInfo{name: "evil:@006c6f6c2f:mwahaha", size: 0, mode: 0755 | os.ModeDir})

			fis, err = ioutil.ReadDir(filepath.Join(mntpath, "evil:@006c6f6c2f:mwahaha"))
			if err != nil {
				t.Fatal(err)
			}
			if g, e := len(fis), 1; g != e {
				t.Fatalf("wrong readdir results: got %v", fis)
			}
			checkFI(t, fis[0], fileInfo{name: "@0102:foobar", size: 5, mode: 0644})
		})
	})
}

func TestRoundtrip(t *testing.T) {
	withDB(t, func(db *bolt.DB) {
		prep := func(tx *bolt.Tx) error {
			_, err := tx.CreateBucket([]byte("bukkit"))
			if err != nil {
				return err
			}
			return nil
		}
		if err := db.Update(prep); err != nil {
			t.Fatal(err)
		}
		withMount(t, db, func(mntpath string) {
			if err := ioutil.WriteFile(
				filepath.Join(mntpath, "bukkit", "greeting"),
				[]byte("hello"),
				0600,
			); err != nil {
				t.Fatal(err)
			}

			data, err := ioutil.ReadFile(filepath.Join(mntpath, "bukkit", "greeting"))
			if err != nil {
				t.Fatal(err)
			}
			if g, e := string(data), "hello"; g != e {
				t.Fatalf("wrong read results: %q != %q", g, e)
			}
		})
	})
}

func TestUnifiedBuffer(t *testing.T) {
	withDB(t, func(db *bolt.DB) {
		prep := func(tx *bolt.Tx) error {
			_, err := tx.CreateBucket([]byte("bukkit"))
			if err != nil {
				return err
			}
			return nil
		}
		if err := db.Update(prep); err != nil {
			t.Fatal(err)
		}
		withMount(t, db, func(mntpath string) {
			f, err := os.Create(filepath.Join(mntpath, "bukkit", "greeting"))
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()
			if _, err := f.Write([]byte("hello")); err != nil {
				t.Fatal(err)
			}
			data, err := ioutil.ReadFile(filepath.Join(mntpath, "bukkit", "greeting"))
			if err != nil {
				t.Fatal(err)
			}
			if g, e := string(data), "hello"; g != e {
				t.Fatalf("wrong read results: %q != %q", g, e)
			}
		})
	})
}

func TestReadTwice(t *testing.T) {
	// catches a bug where Flush on a read-only handle emptied the
	// file
	withDB(t, func(db *bolt.DB) {
		prep := func(tx *bolt.Tx) error {
			b, err := tx.CreateBucket([]byte("bukkit"))
			if err != nil {
				return err
			}
			if err := b.Put([]byte("greeting"), []byte("hello")); err != nil {
				return err
			}
			return nil
		}
		if err := db.Update(prep); err != nil {
			t.Fatal(err)
		}
		withMount(t, db, func(mntpath string) {
			data, err := ioutil.ReadFile(filepath.Join(mntpath, "bukkit", "greeting"))
			if err != nil {
				t.Fatal(err)
			}
			if g, e := string(data), "hello"; g != e {
				t.Fatalf("wrong read results: %q != %q", g, e)
			}
		})
		check := func(tx *bolt.Tx) error {
			b := tx.Bucket([]byte("bukkit"))
			if b == nil {
				t.Fatalf("bukkit disappeared")
			}
			v := b.Get([]byte("greeting"))
			if g, e := string(v), "hello"; g != e {
				t.Fatalf("wrong write content: %q != %q", g, e)
			}
			return nil
		}
		if err := db.View(check); err != nil {
			t.Fatal(err)
		}
	})
}

func TestSeekAndWrite(t *testing.T) {
	withDB(t, func(db *bolt.DB) {
		prep := func(tx *bolt.Tx) error {
			_, err := tx.CreateBucket([]byte("bukkit"))
			if err != nil {
				return err
			}
			return nil
		}
		if err := db.Update(prep); err != nil {
			t.Fatal(err)
		}
		withMount(t, db, func(mntpath string) {
			f, err := os.Create(
				filepath.Join(mntpath, "bukkit", "greeting"),
			)
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()
			n, err := f.WriteAt([]byte("offset"), 3)
			if err != nil {
				t.Fatal(err)
			}
			if g, e := n, 6; g != e {
				t.Errorf("bad write length: %d != %d", g, e)
			}
		})
		check := func(tx *bolt.Tx) error {
			b := tx.Bucket([]byte("bukkit"))
			if b == nil {
				t.Fatalf("bukkit disappeared")
			}
			v := b.Get([]byte("greeting"))
			if g, e := string(v), "\x00\x00\x00offset"; g != e {
				t.Fatalf("wrong write content: %q != %q", g, e)
			}
			return nil
		}
		if err := db.View(check); err != nil {
			t.Fatal(err)
		}
	})
}

func TestTruncate(t *testing.T) {
	withDB(t, func(db *bolt.DB) {
		prep := func(tx *bolt.Tx) error {
			_, err := tx.CreateBucket([]byte("bukkit"))
			if err != nil {
				return err
			}
			return nil
		}
		if err := db.Update(prep); err != nil {
			t.Fatal(err)
		}
		withMount(t, db, func(mntpath string) {
			f, err := os.Create(
				filepath.Join(mntpath, "bukkit", "greeting"),
			)
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()
			if err := f.Truncate(3); err != nil {
				t.Fatal(err)
			}
		})
		check := func(tx *bolt.Tx) error {
			b := tx.Bucket([]byte("bukkit"))
			if b == nil {
				t.Fatalf("bukkit disappeared")
			}
			v := b.Get([]byte("greeting"))
			if g, e := string(v), "\x00\x00\x00"; g != e {
				t.Fatalf("wrong write content: %q != %q", g, e)
			}
			return nil
		}
		if err := db.View(check); err != nil {
			t.Fatal(err)
		}
	})
}
