package main

import (
	"flag"
	"fmt"
	"os"
	"time"

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
	out.root.attr.Mode = fuse.S_IFDIR | 0755
	return out
}

type inMemFS struct {
	root *inMemNode
}

func (fs *inMemFS) createNode() *inMemNode {
	node := &inMemNode{fs: fs,}
	now := time.Now()
	node.attr.SetTimes(&now, &now, &now)
	return node
}

type inMemNode struct {
	fs *inMemFS
	inode *nodefs.Inode
	attr fuse.Attr
}

func (node *inMemNode) Inode() *nodefs.Inode {
	return node.inode
}

func (node *inMemNode) SetInode(inode *nodefs.Inode) {
	node.inode = inode
}

func (node *inMemNode) OnMount(conn *nodefs.FileSystemConnector) {

}

func (node *inMemNode) OnUnmount() {

}

func (parent *inMemNode) Lookup(out *fuse.Attr, name string, context *fuse.Context) (*nodefs.Inode, fuse.Status) {
	child := parent.inode.GetChild(name)
	if child != nil {
		if inMemChild, success := child.Node().(*inMemNode); success {
			*out = inMemChild.attr
		}
		return child, fuse.OK
	}
	return nil, fuse.ENOENT
}

func (node *inMemNode) Deletable() bool {
	return true
}

func (node *inMemNode) OnForget() {

}

func (node *inMemNode) Access(mode uint32, context *fuse.Context) (code fuse.Status) {
	return fuse.ENOSYS
}

func (node *inMemNode) Readlink(c *fuse.Context) ([]byte, fuse.Status) {
	return nil, fuse.ENOSYS
}

func (node *inMemNode) Mknod(name string, mode uint32, dev uint32, context *fuse.Context) (newNode *nodefs.Inode, code fuse.Status) {
	return nil, fuse.ENOSYS
}

func (parent *inMemNode) Mkdir(name string, mode uint32, context *fuse.Context) (newNode *nodefs.Inode, code fuse.Status) {
	node := parent.fs.createNode()
	node.attr.Mode = mode | fuse.S_IFDIR
	inode := parent.inode.NewChild(name, true, node)
	return inode, fuse.OK
}

func (node *inMemNode) Unlink(name string, context *fuse.Context) (code fuse.Status) {
	return fuse.ENOSYS
}

func (node *inMemNode) Rmdir(name string, context *fuse.Context) (code fuse.Status) {
	return fuse.ENOSYS
}

func (node *inMemNode) Symlink(name string, content string, context *fuse.Context) (*nodefs.Inode, fuse.Status) {
	return nil, fuse.ENOSYS
}

func (node *inMemNode) Rename(oldName string, newParent nodefs.Node, newName string, context *fuse.Context) (code fuse.Status) {
	return fuse.ENOSYS
}

func (node *inMemNode) Link(name string, existing nodefs.Node, context *fuse.Context) (newNode *nodefs.Inode, code fuse.Status) {
	return nil, fuse.ENOSYS
}

func (node *inMemNode) Create(name string, flags uint32, mode uint32, context *fuse.Context) (file nodefs.File, child *nodefs.Inode, code fuse.Status) {
	return nil, nil, fuse.ENOSYS
}

func (node *inMemNode) Open(flags uint32, context *fuse.Context) (file nodefs.File, code fuse.Status) {
	return nil, fuse.ENOSYS
}

func (node *inMemNode) OpenDir(context *fuse.Context) ([]fuse.DirEntry, fuse.Status) {
	children := node.inode.FsChildren()
	ls := make([]fuse.DirEntry, 0, len(children))
	for name, inode := range children {
		if childNode, err := inode.Node().(*inMemNode); err {
			ls = append(ls, fuse.DirEntry{Name: name, Mode: childNode.attr.Mode})
		}
	}
	return ls, fuse.OK
}

func (node *inMemNode) Read(file nodefs.File, dest []byte, off int64, context *fuse.Context) (fuse.ReadResult, fuse.Status) {
	return nil, fuse.ENOSYS
}

func (node *inMemNode) Write(file nodefs.File, data []byte, off int64, context *fuse.Context) (written uint32, code fuse.Status) {
	return 0, fuse.ENOSYS
}

func (node *inMemNode) GetXAttr(attribute string, context *fuse.Context) (data []byte, code fuse.Status) {
	return nil, fuse.ENOSYS
}

func (node *inMemNode) RemoveXAttr(attr string, context *fuse.Context) fuse.Status {
	return fuse.ENOSYS
}

func (node *inMemNode) SetXAttr(attr string, data []byte, flags int, context *fuse.Context) fuse.Status {
	return fuse.ENOSYS
}

func (node *inMemNode) ListXAttr(context *fuse.Context) (attrs []string, code fuse.Status) {
	return nil, fuse.ENOSYS
}

func (node *inMemNode) GetAttr(out *fuse.Attr, file nodefs.File, context *fuse.Context) (code fuse.Status) {
	*out = node.attr
	return fuse.OK
}

func (node *inMemNode) Chmod(file nodefs.File, perms uint32, context *fuse.Context) (code fuse.Status) {
	return fuse.ENOSYS
}

func (node *inMemNode) Chown(file nodefs.File, uid uint32, gid uint32, context *fuse.Context) (code fuse.Status) {
	return fuse.ENOSYS
}

func (node *inMemNode) Truncate(file nodefs.File, size uint64, context *fuse.Context) (code fuse.Status) {
	return fuse.ENOSYS
}

func (node *inMemNode) Utimens(file nodefs.File, atime *time.Time, mtime *time.Time, context *fuse.Context) (code fuse.Status) {
	return fuse.ENOSYS
}

func (node *inMemNode) Fallocate(file nodefs.File, off uint64, size uint64, mode uint32, context *fuse.Context) (code fuse.Status) {
	return fuse.ENOSYS
}

func (node *inMemNode) StatFs() *fuse.StatfsOut {
	return &fuse.StatfsOut{}
}








