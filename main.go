package main

import (
	"flag"
	"fmt"
	"os"
	"sync"
	"time"
	"syscall"

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
	fs := &inMemFS{}
	fs.root = fs.createNode()
	fs.root.attr.Mode = fuse.S_IFDIR | 0755
	fs.root.attr.Nlink = 2
	fs.blockSize = 4096
	return fs
}

type inMemFS struct {
	root *inMemNode
	blockSize uint32
}

func (fs *inMemFS) createNode() *inMemNode {
	node := &inMemNode{fs: fs,}
	now := time.Now()
	node.attr.SetTimes(&now, &now, &now)
	node.attr.Nlink = 1
	node.attr.Blksize = fs.blockSize
	node.contents = make([]byte, 0)
	return node
}

type inMemNode struct {
	fs *inMemFS
	inode *nodefs.Inode
	metadataMutex sync.RWMutex
	attr fuse.Attr
	xattr map[string][]byte
	contents []byte
}

func (node *inMemNode) incrementLinks() {
	node.metadataMutex.Lock()
	node.attr.Nlink += 1
	node.metadataMutex.Unlock()
}

func (node *inMemNode) decrementLinks() {
	node.metadataMutex.Lock()
	node.attr.Nlink -= 1
	node.metadataMutex.Unlock()
}

func (node *inMemNode) Inode() *nodefs.Inode {
	return node.inode
}

func (node *inMemNode) SetInode(inode *nodefs.Inode) {
	node.inode = inode
}

func (node *inMemNode) OnMount(conn *nodefs.FileSystemConnector) {
	fmt.Printf("Mounted\n")
}

func (node *inMemNode) OnUnmount() {
	fmt.Printf("Unmounted\n")
}

func (parent *inMemNode) Lookup(out *fuse.Attr, name string, context *fuse.Context) (*nodefs.Inode, fuse.Status) {
	child := parent.inode.GetChild(name)
	if child != nil {
		if inMemChild, success := child.Node().(*inMemNode); success {
			inMemChild.metadataMutex.RLock()
			*out = inMemChild.attr
			inMemChild.metadataMutex.RUnlock()
		}
		return child, fuse.OK
	}
	return nil, fuse.ENOENT
}

func (node *inMemNode) Deletable() bool {
	return true
}

func (node *inMemNode) OnForget() {
	node.setSize(0)
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
	node.attr.Nlink = 2
	inode := parent.inode.NewChild(name, true, node)
	parent.incrementLinks()
	return inode, fuse.OK
}

func (node *inMemNode) Unlink(name string, context *fuse.Context) (code fuse.Status) {
	child := node.Inode().GetChild(name)
	if(child == nil) {
		return fuse.ENOENT
	}
	node.Inode().RmChild(name)
	if inMemChild, ok := child.Node().(*inMemNode); ok {
		inMemChild.decrementLinks()
		inMemChild.metadataMutex.RLock()
		if inMemChild.attr.Nlink == 0 {
			node.decrementLinks()
		}
		inMemChild.metadataMutex.RUnlock()
	}
	return fuse.OK
}

func (node *inMemNode) Rmdir(name string, context *fuse.Context) (code fuse.Status) {
	return node.Unlink(name, context)
}

func (node *inMemNode) Symlink(name string, content string, context *fuse.Context) (*nodefs.Inode, fuse.Status) {
	return nil, fuse.ENOSYS
}

func (parent *inMemNode) Rename(oldName string, newParent nodefs.Node, newName string, context *fuse.Context) (code fuse.Status) {
	child := parent.Inode().GetChild(oldName)
	if(child == nil) {
		return fuse.ENOENT
	}
	parent.Inode().RmChild(oldName)
	parent.decrementLinks()
	newParent.Inode().RmChild(newName)
	newParent.Inode().AddChild(newName, child)
	if inMemNewParent, ok := newParent.(*inMemNode); ok {
		inMemNewParent.incrementLinks()
	}
	return fuse.OK
}

func (node *inMemNode) Link(name string, existing nodefs.Node, context *fuse.Context) (newNode *nodefs.Inode, code fuse.Status) {
	if node.Inode().GetChild(name) != nil {
		return nil, fuse.Status(syscall.EEXIST)
	}
	node.Inode().AddChild(name, existing.Inode())
	if inMemChild, ok := existing.(*inMemNode); ok {
		inMemChild.incrementLinks()
	}
	return existing.Inode(), fuse.OK
}

func (parent *inMemNode) Create(name string, flags uint32, mode uint32, context *fuse.Context) (file nodefs.File, child *nodefs.Inode, code fuse.Status) {
	if parent.Inode().GetChild(name) != nil {
		return nil, nil, fuse.Status(syscall.EEXIST)
	}
	node := parent.fs.createNode()
	node.attr.Mode = mode | fuse.S_IFREG
	parent.Inode().NewChild(name, false, node)
	parent.incrementLinks()
	f, openStatus := node.Open(flags, context)
	if openStatus != fuse.OK {
		return nil, nil, openStatus
	}
	return f, node.Inode(), fuse.OK
}

func (node *inMemNode) Open(flags uint32, context *fuse.Context) (file nodefs.File, code fuse.Status) {
	f := node.createFile()
	f.flags = flags
	return f, fuse.OK
}

func (node *inMemNode) OpenDir(context *fuse.Context) ([]fuse.DirEntry, fuse.Status) {
	children := node.inode.FsChildren()
	ls := make([]fuse.DirEntry, 0, len(children))
	for name, inode := range children {
		if childNode, success := inode.Node().(*inMemNode); success {
			childNode.metadataMutex.RLock()
			ls = append(ls, fuse.DirEntry{Name: name, Mode: childNode.attr.Mode})
			childNode.metadataMutex.RUnlock()
		}
	}
	return ls, fuse.OK
}

func (node *inMemNode) Read(file nodefs.File, dest []byte, off int64, context *fuse.Context) (fuse.ReadResult, fuse.Status) {
	if int(off) > len(node.contents) {
		return nil, fuse.EIO
	}
	if len(dest) > len(node.contents) - int(off) {
		dest = dest[:len(node.contents) - int(off)]
	}
	copy(dest, node.contents[off:])
	return fuse.ReadResultData(dest), fuse.OK
}

func (node *inMemNode) setSize(size int) {
	newContents := make([]byte, size)
	if len(node.contents) > size {
		copy(newContents, node.contents[:size])
	} else {
		copy(newContents, node.contents)
	}
	node.metadataMutex.Lock()
	node.contents = newContents
	node.attr.Size = uint64(size)
	node.attr.Blocks = uint64(size / 512)
	if size % 512 > 0 {
		node.attr.Blocks += 1
	}
	node.metadataMutex.Unlock()
}

func (node *inMemNode) Write(file nodefs.File, data []byte, off int64, context *fuse.Context) (written uint32, code fuse.Status) {
	if len(node.contents) < len(data) + int(off) {
		node.setSize(len(data) + int(off))
	}
	copy(node.contents[off:], data)
	return uint32(len(data)), fuse.OK
}

func (node *inMemNode) GetXAttr(attribute string, context *fuse.Context) (data []byte, code fuse.Status) {
	node.metadataMutex.RLock()
	xattr := node.xattr[attribute]
	node.metadataMutex.RUnlock()
	if xattr == nil {
		return nil, fuse.ENODATA
	}
	return xattr, fuse.OK
}

func (node *inMemNode) RemoveXAttr(attr string, context *fuse.Context) fuse.Status {
	node.metadataMutex.Lock()
	xattr := node.xattr[attr]
	delete(node.xattr, attr)
	node.metadataMutex.Unlock()
	if xattr == nil {
		return fuse.ENODATA
	}
	return fuse.OK
}

func (node *inMemNode) SetXAttr(attr string, data []byte, flags int, context *fuse.Context) fuse.Status {
	node.metadataMutex.Lock()
	node.xattr[attr] = data
	node.metadataMutex.Unlock()
	return fuse.OK
}

func (node *inMemNode) ListXAttr(context *fuse.Context) (attrs []string, code fuse.Status) {
	return nil, fuse.ENOSYS
}

func (node *inMemNode) GetAttr(out *fuse.Attr, file nodefs.File, context *fuse.Context) (code fuse.Status) {
	node.metadataMutex.RLock()
	*out = node.attr
	node.metadataMutex.RUnlock()
	return fuse.OK
}

func setBit(attr *uint32, mask uint32, field uint32) {
	*attr &= ^mask
	*attr |= (mask & field)
}

func (node *inMemNode) Chmod(file nodefs.File, perms uint32, context *fuse.Context) (code fuse.Status) {
	node.metadataMutex.Lock()
	setBit(&node.attr.Mode, syscall.S_IRUSR, perms)
	setBit(&node.attr.Mode, syscall.S_IWUSR, perms)
	setBit(&node.attr.Mode, syscall.S_IXUSR, perms)
	setBit(&node.attr.Mode, syscall.S_IRGRP, perms)
	setBit(&node.attr.Mode, syscall.S_IWGRP, perms)
	setBit(&node.attr.Mode, syscall.S_IXGRP, perms)
	setBit(&node.attr.Mode, syscall.S_IROTH, perms)
	setBit(&node.attr.Mode, syscall.S_IWOTH, perms)
	setBit(&node.attr.Mode, syscall.S_IXOTH, perms)
	node.metadataMutex.Unlock()
	return fuse.OK
}

func (node *inMemNode) Chown(file nodefs.File, uid uint32, gid uint32, context *fuse.Context) (code fuse.Status) {
	node.metadataMutex.Lock()
	node.attr.Uid = uid
	node.attr.Gid = gid
	node.metadataMutex.Unlock()
	return fuse.OK
}

func (node *inMemNode) Truncate(file nodefs.File, size uint64, context *fuse.Context) (code fuse.Status) {
	node.setSize(int(size))
	return fuse.OK
}

func (node *inMemNode) Utimens(file nodefs.File, atime *time.Time, mtime *time.Time, context *fuse.Context) (code fuse.Status) {
	node.metadataMutex.Lock()
	changeTime := node.attr.ChangeTime()
	node.attr.SetTimes(atime, mtime, &changeTime)
	node.metadataMutex.Unlock()
	return fuse.OK
}

func (node *inMemNode) Fallocate(file nodefs.File, off uint64, size uint64, mode uint32, context *fuse.Context) (code fuse.Status) {
	return fuse.ENOSYS
}

func (node *inMemNode) StatFs() *fuse.StatfsOut {
	return &fuse.StatfsOut{}
}

var _ nodefs.File = (*inMemFile)(nil)

type inMemFile struct {
	node *inMemNode
	flags uint32
}

func (node *inMemNode) createFile() *inMemFile {
	return &inMemFile{node: node}
}

// Called upon registering the filehandle in the inode.
func (f *inMemFile) SetInode(inode *nodefs.Inode) {
	if f.node.inode != inode {
		panic("inMemFile: wrong inode detected")
	}
}

// The String method is for debug printing.
func (f *inMemFile) String() string {
	return fmt.Sprintf("inMemFile")
}

// Wrappers around other File implementations, should return
// the inner file here.
func (f *inMemFile) InnerFile() nodefs.File {
	return nil
}

func (f *inMemFile) Read(dest []byte, off int64) (fuse.ReadResult, fuse.Status){
	return nil, fuse.ENOSYS
}

func (f *inMemFile) Write(data []byte, off int64) (written uint32, code fuse.Status) {
	return uint32(len(data)), fuse.OK
}

// Flush is called for close() call on a file descriptor. In
// case of duplicated descriptor, it may be called more than
// once for a file.
func (f *inMemFile) Flush() fuse.Status {
	return fuse.OK
}

// This is called to before the file handle is forgotten. This
// method has no return value, so nothing can synchronizes on
// the call. Any cleanup that requires specific synchronization or
// could fail with I/O errors should happen in Flush instead.
func (f *inMemFile) Release() {

}

func (f *inMemFile) Fsync(flags int) (code fuse.Status) {
	return fuse.OK
}

// The methods below may be called on closed files, due to
// concurrency.  In that case, you should return EBADF.
func (f *inMemFile) Truncate(size uint64) fuse.Status {
	return fuse.ENOSYS
}

func (f *inMemFile) GetAttr(out *fuse.Attr) fuse.Status {
	return f.node.GetAttr(out, f, nil)
}

func (f *inMemFile) Chown(uid uint32, gid uint32) fuse.Status {
	return f.node.Chown(f, uid, gid, nil)
}

func (f *inMemFile) Chmod(perms uint32) fuse.Status {
	return f.node.Chmod(f, perms, nil)
}

func (f *inMemFile) Utimens(atime *time.Time, mtime *time.Time) fuse.Status {
	return f.node.Utimens(f, atime, mtime, nil)
}

func (f *inMemFile) Allocate(off uint64, size uint64, mode uint32) (code fuse.Status) {
	return f.node.Fallocate(f, off, size, mode, nil)
}








