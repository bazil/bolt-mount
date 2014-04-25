package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
)

var progName = filepath.Base(os.Args[0])

func usage() {
	fmt.Fprintf(os.Stderr, "Usage of %s:\n", progName)
	fmt.Fprintf(os.Stderr, "  %s DBPATH MOUNTPOINT\n", progName)
	flag.PrintDefaults()
}

func main() {
	log.SetFlags(0)
	log.SetPrefix(progName + ": ")

	flag.Usage = usage
	flag.Parse()

	if flag.NArg() != 2 {
		usage()
		os.Exit(2)
	}

	err := mount(flag.Arg(0), flag.Arg(1))
	if err != nil {
		log.Fatal(err)
	}
}
