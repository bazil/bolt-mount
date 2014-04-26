# Mount a Bolt database as a FUSE filesystem

[Bolt](https://github.com/boltdb/bolt) is a key-value store that also
supports nested buckets. This makes it look a little bit like a file
system tree.

`bolt-mount` exposes a Bolt database as a FUSE file system.

``` console
$ go get bazil.org/bolt-mount
# assuming $GOPATH/bin is in $PATH
$ mkdir mnt
$ bolt-mount mydatabase.bolt mnt &
$ cd mnt
$ mkdir bucket
$ mkdir bucket/sub
$ echo Hello, world >bucket/sub/greeting
$ ls -l bucket
total 0
drwxr-xr-x 1 root root 0 Apr 25 18:00 sub/
$ ls -l bucket/sub
total 0
-rw-r--r-- 1 root root 0 Apr 25 18:00 greeting
$ cat bucket/sub/greeting
Hello, world
$ cd ..
# for Linux
$ fusermount -u mnt
# for OS X
$ umount mnt
[1]+  Done                    bolt-mount mydatabase.bolt mnt
```

## Encoding keys to file names

As Bolt keys can contain arbitrary bytes, but file names cannot, the
keys are encoded.

First, we define *safe* as:

- ASCII letters and numbers
- the characters ".", "," "-", "_" (period/dot, comma, dash, underscore)

A name consisting completely of *safe* characters, and not starting
with a dot, is unaltered. Everything else is hex-encoded. Hex encoding
looks like `@xx[xx..]` where `xx` are lower case hex digits.

Additionally, any *safe* prefixes (not starting with a dot) and
suffixes longer than than a noise threshold remain unaltered. They are
separated from the hex encoded middle part by a semicolon, as in
`[PREFIX:]MIDDLE[:SUFFIX]`.

For example:

A Bolt key packing two little-endian `uint16` values 42 and 10000 and the string
"test" is encoded as filename `@002a2710:test`.
