package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/hanwen/go-fuse/fuse"
	"github.com/hanwen/go-fuse/fuse/nodefs"
)

// this function was borrowed from https://raw.githubusercontent.com/hanwen/go-fuse/master/example/memfs/main.go
func main() {
	// Scans the arg list and sets up flags
	debug := flag.Bool("debug", false, "print debugging messages.")
	flag.Parse()
	if flag.NArg() < 1 {
		fmt.Println("usage: inmemfs MOUNTPOINT")
		os.Exit(2)
	}

	mountPoint := flag.Arg(0)
	root := newInMemFS().root
	conn := nodefs.NewFileSystemConnector(root, nil)
	server, err := fuse.NewServer(conn.RawFS(), mountPoint, nil)
	if err != nil {
		fmt.Printf("Mount fail: %v\n", err)
		os.Exit(1)
	}
	server.SetDebug(*debug)
	fmt.Println("Mounted!")
	server.Serve()
}

func newInMemFS() *inMemFS {
	out := &inMemFS{}
	out.root = out.createNode()
	return out
}

type inMemFS struct {
	root *inMemNode
}

func (fs *inMemFS) createNode() *inMemNode {
	out := &inMemNode{Node: nodefs.NewDefaultNode(), fs: fs,}
	return out
}

type inMemNode struct {
	nodefs.Node
	fs *inMemFS
}
