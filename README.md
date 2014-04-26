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
