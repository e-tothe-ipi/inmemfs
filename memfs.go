package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"bazil.org/fuse"
	"bazil.org/fuse/fs"
	"golang.org/x/net/context"
)

// This first part is mostly taken from the clockfs example
func usage() {
	fmt.Fprintf(os.Stderr, "Usage of %s:\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "  %s MOUNTPOINT\n", os.Args[0])
	flag.PrintDefaults()
}

func run(mountpoint string) error {
	c, err := fuse.Mount(
		mountpoint,
		fuse.FSName("inmemfs"),
		fuse.Subtype("inmemfsfs"),
		fuse.LocalVolume(),
		fuse.VolumeName("Memory filesystem"),
	)
	if err != nil {
		return err
	}
	defer c.Close()

	if p := c.Protocol(); !p.HasInvalidate() {
		return fmt.Errorf("kernel FUSE support is too old to have invalidations: version %v", p)
	}

	srv := fs.New(c, nil)
	filesys := &MemFS{ lastInode: 1, lastInodeLock: &sync.Mutex{}, }
	filesys.root = &MemNode { fs: filesys, inode: 1, mode: 0555, dir: true, name: "", children: []*MemNode{},
								atime: time.Now(), mtime: time.Now(), ctime: time.Now(), contents: make([]byte, 0)}
	if err := srv.Serve(filesys); err != nil {
		return err
	}

	// Check if the mount process has an error to report.
	<-c.Ready
	if err := c.MountError; err != nil {
		return err
	}
	return nil
}

func main() {
	flag.Usage = usage
	flag.Parse()

	if flag.NArg() != 1 {
		usage()
		os.Exit(2)
	}
	mountpoint := flag.Arg(0)

	if err := run(mountpoint); err != nil {
		log.Fatal(err)
	}
}




type MemFS struct {
	root *MemNode
	lastInode uint64
	lastInodeLock *sync.Mutex
}

func (fs *MemFS) Root() (fs.Node, error) {
	return fs.root, nil
}

func (fs *MemFS) GenerateInode(parentInode uint64, name string) uint64 {
	return fs.NextInode()
}

func (fs *MemFS) NextInode() uint64 {
	fs.lastInodeLock.Lock()
	var next uint64 = fs.lastInode + 1
	fs.lastInode = next
	fs.lastInodeLock.Unlock()
	return next
}


func (fs *MemFS) Statfs(ctx context.Context, req *fuse.StatfsRequest, resp *fuse.StatfsResponse) error {
	resp.Blocks = 100000
	resp.Bfree = 100000
	resp.Bavail = 100000
	resp.Files = 100000
	resp.Ffree = 100000
	resp.Bsize = 4096
	resp.Namelen = 1024
	resp.Frsize = 512
	return nil
}


type MemNode struct {
	fs		*MemFS
	parent	*MemNode
	inode	uint64
	mode	os.FileMode
	dir		bool
	children []*MemNode
	name	string
	atime	time.Time
	mtime	time.Time
	ctime	time.Time
	uid		uint32
	gid		uint32
	contents []byte
}

func (node *MemNode) String() string {
	return fmt.Sprintf("MemNode %d %q mode=%d,dir=%t,len(children)=%d", node.inode, node.name, node.mode, node.dir, len(node.children))
}

func (node *MemNode) Attr(ctx context.Context, a *fuse.Attr) error {
	a.Inode = node.inode
	if node.dir {
		a.Mode = os.ModeDir
	}
	a.Mode |= node.mode
	a.Valid = time.Duration(0)
	a.Size = node.size()
	a.Blocks = node.blocks()
	a.Atime = node.atime
	a.Mtime = node.mtime
	a.Ctime = node.ctime
	a.Uid = node.uid
	a.Gid = node.gid
	return nil
}

func (node *MemNode) size() uint64 {
	return uint64(len(node.contents))
}

func (node *MemNode) blocks() uint64 {
	if node.size() % 512 == 0 {
		return node.size() / 512
	}
	return (node.size() / 512) + 1
}

func (node *MemNode) find(name string) *MemNode {
	for _, n := range node.children {
		if n.name == name {
			return n
		}
	}
	return nil
}

func (node *MemNode) add(child *MemNode) {
	node.children = append(node.children, child)
	node.ctime = time.Now()
}

func (node *MemNode) remove(child *MemNode) {
	newChildren := make([]*MemNode,0)
	for _, n := range node.children {
		if n != child {
			newChildren = append(newChildren, n)
		}
	}
	node.children = newChildren
	node.ctime = time.Now()
}

func (node *MemNode) changeSize(size int) {
	newContents := make([]byte, size)
	if size < len(node.contents) {
		copy(newContents, node.contents[:size])
	} else {
		copy(newContents, node.contents)
	}
	node.contents = newContents
}

func (node *MemNode) Mkdir(ctx context.Context, req *fuse.MkdirRequest) (fs.Node, error) {
	if node.find(req.Name) != nil {
		return nil, fuse.EEXIST
	}
	newNode := new(MemNode)
	newNode.fs = node.fs
	newNode.parent = node
	newNode.inode = node.fs.NextInode()
	newNode.mode = req.Mode
	newNode.dir = true
	newNode.name = req.Name
	newNode.contents = make([]byte, 0)
	newNode.atime = time.Now()
	newNode.mtime = time.Now()
	newNode.ctime = time.Now()
	newNode.uid = req.Header.Uid
	newNode.gid = req.Header.Gid
	node.add(newNode)
	return newNode, nil
}

func (node *MemNode) Remove(ctx context.Context, req *fuse.RemoveRequest) error {
	n := node.find(req.Name)
	if n == nil {
		return fuse.ENOENT
	}
	node.remove(n)
	return nil
}

func (node *MemNode) Lookup(ctx context.Context, name string) (fs.Node, error) {
	n := node.find(name)
	if n != nil {
		return n, nil
	}
	return nil, fuse.ENOENT
}

func (node *MemNode) Open(ctx context.Context, req *fuse.OpenRequest, resp *fuse.OpenResponse) (fs.Handle, error) {
	if req.Dir && !node.dir {
		return nil, fuse.ENOENT
	}
	return node, nil
}



var _ fs.HandleReadDirAller = (*MemNode)(nil)

func (node *MemNode) ReadDirAll(ctx context.Context) ([]fuse.Dirent, error) {
	out := make([]fuse.Dirent, 0)
	dot := fuse.Dirent{Inode: node.inode, Type: fuse.DT_Dir, Name: "."}
	out = append(out, dot)
	var ddot fuse.Dirent
	if node.parent != nil {
		ddot = fuse.Dirent{Inode: node.parent.inode, Type: fuse.DT_Dir, Name: ".."}
	} else {
		ddot = fuse.Dirent{Inode: node.inode, Type: fuse.DT_Dir, Name: ".."}
	}
	out = append(out, ddot)
	for _, n := range node.children {
		item := fuse.Dirent{}
		item.Inode = n.inode
		if n.dir {
			item.Type = fuse.DT_Dir
		} else {
			item.Type = fuse.DT_File
		}
		item.Name = n.name
		out = append(out, item)
	}
	return out, nil
}

func (node *MemNode) Rename(ctx context.Context, req *fuse.RenameRequest, newDir fs.Node) error {
	item := node.find(req.OldName)
	item.name = req.NewName
	if newMemDir, ok := newDir.(*MemNode); ok {
		if(node.inode != newMemDir.inode) {
			node.remove(item)
			newMemDir.add(item)
		}
	}
	return nil
}


func (node *MemNode) Setattr(ctx context.Context, req *fuse.SetattrRequest, resp *fuse.SetattrResponse) error {
	if req.Valid.Atime() {
		node.atime = req.Atime
	}
	if req.Valid.Mtime() {
		node.mtime = req.Mtime
	}
	if req.Valid.Mode() {
		node.mode = req.Mode
	}
	if req.Valid.Uid() {
		node.uid = req.Uid
	}
	if req.Valid.Gid() {
		node.gid = req.Gid
	}
	if req.Valid.Size() {
		node.changeSize(int(req.Size))
	}
	return nil
}

func (node *MemNode) Create(ctx context.Context, req *fuse.CreateRequest, resp *fuse.CreateResponse) (fs.Node, fs.Handle, error) {
	if node.find(req.Name) != nil {
		return nil, nil, fuse.EEXIST
	}
	newNode := new(MemNode)
	newNode.fs = node.fs
	newNode.parent = node
	newNode.inode = node.fs.NextInode()
	newNode.mode = req.Mode
	newNode.dir = false
	newNode.name = req.Name
	newNode.contents = make([]byte, 0)
	newNode.atime = time.Now()
	newNode.mtime = time.Now()
	newNode.ctime = time.Now()
	newNode.uid = req.Header.Uid
	newNode.gid = req.Header.Gid
	node.add(newNode)
	return newNode, newNode, nil
}

func (node *MemNode) Read(ctx context.Context, req *fuse.ReadRequest, resp *fuse.ReadResponse) error {
	if !(req.FileFlags.IsReadOnly() || req.FileFlags.IsReadWrite()) {
		return fuse.EPERM
	}
	readEnd := int(req.Offset) + req.Size
	if readEnd > len(node.contents) {
		readEnd = len(node.contents)
	}
	resp.Data = node.contents[req.Offset:readEnd]
	return nil
}

func (node *MemNode) Write(ctx context.Context, req *fuse.WriteRequest, resp *fuse.WriteResponse) error {
	if !(req.FileFlags.IsReadWrite() || req.FileFlags.IsWriteOnly()) {
		return fuse.EPERM
	}
	newLen := int(req.Offset) + len(req.Data)
	if newLen > len(node.contents) {
		node.changeSize(newLen)
	}
	copy(node.contents[req.Offset:], req.Data)
	resp.Size = len(req.Data)
	return nil
}

