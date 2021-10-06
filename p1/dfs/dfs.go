// memfs implements a simple in-memory file system.  v0.2
package main

/*
 Two main files are ../fuse.go and ../fs/serve.go
*/

import (
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"time"

	. "github.com/mattn/go-getopt"

	"bazil.org/fuse"
	"bazil.org/fuse/fs"
	"golang.org/x/net/context"
)

/*
    Need to implement these types from bazil/fuse/fs!

    type FS interface {
	  // Root is called to obtain the Node for the file system root.
	  Root() (Node, error)
    }

    type Node interface {
	  // Attr fills attr with the standard metadata for the node.
	  Attr(ctx context.Context, attr *fuse.Attr) error
    }
*/

//=============================================================================

var dfs Dfs

type Dfs struct {
	root    *DNode
	nodeMap map[uint64]*DNode
}

type DNode struct {
	//	nid   uint64
	name  string
	attr  fuse.Attr
	dirty bool
	child map[string]*DNode
	data  []uint8
}

func (n *DNode) String() string {
	if n.Type() == fuse.DT_Dir {
		return fmt.Sprintf("(Dir)name=%s, num_child=%d, attr=[%s]", n.name, len(n.child), n.attr)
	} else {
		return fmt.Sprintf("(File)name=%s, size=%d, attr=[%s]", n.name, len(n.data), n.attr)
	}
}

func (n *DNode) Inode() uint64 {
	return n.attr.Inode
}

func (n *DNode) Type() fuse.DirentType {
	if n.child == nil {
		return fuse.DT_File
	} else {
		return fuse.DT_Dir
	}
}

//func (node *DNode) newNode()

//----------------------------------------

//var root *DNode

//  Compile error if DNode does not implement interface fs.Node, or if FS does not implement fs.FS
var _ fs.Node = (*DNode)(nil)
var _ fs.FS = (*Dfs)(nil)

var debug = false
var mountPoint = "/tmp/dss"

//=============================================================================

func p_printf(s string, args ...interface{}) {
	if !debug {
		return
	}
	fmt.Printf(s+"\n", args...)
}

func p_println(args ...interface{}) {
	if !debug {
		return
	}
	fmt.Print(args...)
	fmt.Print("\n")
}

func p_err(s string, args ...interface{}) {
	fmt.Printf(s, args...)
}

//=============================================================================

// Implement:
//   func (Dfs) Root() (n fs.Node, err error)
//   func (n *DNode) Attr(ctx context.Context, attr *fuse.Attr) error
//   func (n *DNode) Lookup(ctx context.Context, name string) (fs.Node, error)
//   func (n *DNode) ReadDirAll(ctx context.Context) ([]fuse.Dirent, error)
//   func (n *DNode) Getattr(ctx context.Context, req *fuse.GetattrRequest, resp *fuse.GetattrResponse) error
// must be defined or editing w/ vi or emacs fails. Doesn't have to do anything

func (dfs Dfs) Root() (n fs.Node, err error) {
	p_println("Root,", dfs.root)
	return dfs.root, nil
}

func (n *DNode) Attr(ctx context.Context, attr *fuse.Attr) error {
	p_printf("Attr, {%s}", n)
	//	attr.Inode = n.attr.Inode
	attr.Mode = n.attr.Mode
	attr.Size = n.attr.Size

	attr.Uid = n.attr.Uid
	attr.Gid = n.attr.Gid
	//TODO: set other attributes
	attr.Atime = n.attr.Atime
	attr.Mtime = n.attr.Mtime
	attr.Ctime = n.attr.Ctime

	return nil
}

func (n *DNode) Lookup(ctx context.Context, name string) (fs.Node, error) {
	p_printf("Lookup, {%s}, %s", n, name)

	n.attr.Atime = time.Now()

	c, ok := n.child[name]
	if !ok {
		p_printf("%s not found", name)
		return nil, fuse.ENOENT
	}
	return c, nil
}

func (n *DNode) ReadDirAll(ctx context.Context) ([]fuse.Dirent, error) {
	p_printf("ReadDirAll, {%s}", n)

	n.attr.Atime = time.Now()

	var result []fuse.Dirent
	if n.Type() != fuse.DT_Dir {
		return nil, errors.New("node is not a directory")
	}
	for _, c := range n.child {
		result = append(result, fuse.Dirent{Inode: c.Inode(), Type: c.Type(), Name: c.name})
	}
	return result, nil
}

func (n *DNode) Getattr(ctx context.Context, req *fuse.GetattrRequest, resp *fuse.GetattrResponse) error {
	p_printf("Getattr, {%s}", n)
	p_println(n.attr)
	resp.Attr = n.attr
	return nil
}

func (n *DNode) Fsync(ctx context.Context, req *fuse.FsyncRequest) error {
	//	p_call("func (n *DNode) Fsync(ctx context.Context, req *fuse.FsyncRequest) error \n")
	p_printf("FSYNC")
	return nil
}

//   func (n *DNode) Setattr(ctx context.Context, req *fuse.SetattrRequest, resp *fuse.SetattrResponse) error
//   func (p *DNode) Mkdir(ctx context.Context, req *fuse.MkdirRequest) (fs.Node, error)
//   func (p *DNode) Create(ctx context.Context, req *fuse.CreateRequest, resp *fuse.CreateResponse) (fs.Node, fs.Handle, error)
//   func (n *DNode) ReadAll(ctx context.Context) ([]byte, error)
//   func (n *DNode) Write(ctx context.Context, req *fuse.WriteRequest, resp *fuse.WriteResponse) error
//   func (n *DNode) Flush(ctx context.Context, req *fuse.FlushRequest) error
//   func (n *DNode) Remove(ctx context.Context, req *fuse.RemoveRequest) error
//   func (n *DNode) Rename(ctx context.Context, req *fuse.RenameRequest, newDir fs.Node) error

func createDir(nid uint64, name string, mode os.FileMode) *DNode {
	now := time.Now()
	return &DNode{
		name:  name,
		attr:  fuse.Attr{Inode: nid, Mode: mode, Nlink: 1, Atime: now, Mtime: now, Ctime: now},
		child: make(map[string]*DNode),
	}
}

func (n *DNode) Mkdir(ctx context.Context, req *fuse.MkdirRequest) (fs.Node, error) {
	p_printf("Mkdir, {%s}, ", n)
	p_println(req)

	//TODO: check if directory?

	if _, ok := n.child[req.Name]; !ok {
		// nid:0 -> dynamic nid
		n.child[req.Name] = createDir(0, req.Name, req.Mode)

	} else {
		return nil, errors.New("directory exists")
	}
	return n.child[req.Name], nil

}

func (n *DNode) Setattr(ctx context.Context, req *fuse.SetattrRequest, resp *fuse.SetattrResponse) error {
	p_printf("Setattr, {%s}", n)
	p_println(req.Mode)
	//TODO req.Valid?
	if req.Valid.Mode() {
		n.attr.Mode = req.Mode
	}
	if req.Valid.Size() {
		n.attr.Size = req.Size
	}

	if req.Valid.Uid() {
		n.attr.Uid = req.Uid
	}
	if req.Valid.Gid() {
		n.attr.Gid = req.Gid
	}

	if req.Valid.Atime() {
		n.attr.Atime = req.Atime
	}

	if req.Valid.Mtime() {
		n.attr.Mtime = req.Mtime
	}
	return nil
}

func (n *DNode) Create(ctx context.Context, req *fuse.CreateRequest, resp *fuse.CreateResponse) (fs.Node, fs.Handle, error) {
	p_printf("Create, {%s}, %s", n, req.Name)

	// req.Flag may need to be implemented

	n.attr.Atime = time.Now()

	var file *DNode
	if _, ok := n.child[req.Name]; !ok {
		// nid:0 -> dyanimc nid
		file = &DNode{
			name: req.Name,
			attr: fuse.Attr{Inode: 0, Mode: req.Mode, Nlink: 1},
			data: make([]uint8, 0)}
		now := time.Now()
		file.attr.Atime = now
		file.attr.Mtime = now
		file.attr.Ctime = now
		n.child[req.Name] = file
	} else {
		return nil, nil, errors.New("file exists")
	}

	n.attr.Mtime = time.Now()

	p_println("Created: ", file)
	return file, file, nil

}

func (n *DNode) ReadAll(ctx context.Context) ([]byte, error) {
	p_printf("ReadAll, %d", n.Inode())
	n.attr.Atime = time.Now()
	return n.data, nil
}

func (n *DNode) Write(ctx context.Context, req *fuse.WriteRequest, resp *fuse.WriteResponse) error {
	p_printf("Write, %d, n.data_size:%d, data_size: %d, offset:%d, ", n.Inode(), len(n.data), len(req.Data), req.Offset)
	// How could this be different?
	resp.Size = len(req.Data)
	if len(req.Data)+int(req.Offset) > len(n.data) {
		tmp := n.data
		n.data = make([]uint8, len(req.Data)+int(req.Offset))
		copy(n.data, tmp)
	}
	copy(n.data[req.Offset:], req.Data)

	// mark as dirty?
	n.dirty = true

	n.attr.Mtime = time.Now()

	n.attr.Size = uint64(len(n.data))
	return nil
}

func (n *DNode) Flush(ctx context.Context, req *fuse.FlushRequest) error {
	p_printf("Flush, {%s}", n)
	if !n.dirty {
		return nil
	}
	now := time.Now()
	n.attr.Atime = now
	n.attr.Mtime = now
	n.dirty = false

	//TODO: change modified time
	return nil
}

func (n *DNode) Remove(ctx context.Context, req *fuse.RemoveRequest) error {
	p_printf("Remove, {%s}", n)

	n.attr.Atime = time.Now()

	var c *DNode
	var ok bool
	if c, ok = n.child[req.Name]; !ok {
		return errors.New("file or dir does not exist")
	}

	if req.Dir && len(c.child) > 0 {
		return errors.New("can not remove an non-empty directory")
	}

	c.attr.Nlink -= 1

	delete(n.child, req.Name)

	n.attr.Mtime = time.Now()

	p_println("Remove successful")
	return nil

}

func (n *DNode) Rename(ctx context.Context, req *fuse.RenameRequest, newDir fs.Node) error {
	p_printf("Rename, {%s}, oldName: %s, newName: %s", n, req.OldName, req.NewName)

	var ok bool
	var node, dest *DNode

	n.attr.Atime = time.Now()
	//TODO use assert when converting
	if dest, ok = newDir.(*DNode); !ok {
		p_printf("can not convert fs.Node to DNode")
		return errors.New("can not convert fs.Node to DNode")
	}

	dest.attr.Atime = time.Now()

	if node, ok = n.child[req.OldName]; !ok {
		p_printf("file or dir does not exist")
		return errors.New("file or dir does not exist")
	}

	/*
		//WOW: didn't know you should overwrite with rename :))
		if _, ok := dest.child[req.NewName]; ok {
			p_printf("file already exists at destination")
			return errors.New("file already exists at destination")
		}
	*/

	node.name = req.NewName
	node.attr.Mtime = time.Now()

	dest.child[node.name] = node
	dest.attr.Mtime = time.Now()

	delete(n.child, req.OldName)
	n.attr.Mtime = time.Now()

	p_printf("Rename successful")

	return nil
}

func (n *DNode) Link(ctx context.Context, req *fuse.LinkRequest, old fs.Node) (fs.Node, error) {
	p_printf("Link, {%s},", n)

	var ok bool
	var node *DNode

	if node, ok = old.(*DNode); !ok {
		p_printf("can not convert fs.Node to DNode")
		return nil, errors.New("can not convert fs.Node to DNode")
	}
	p_printf("newName:%s, oldNode:{%s}", req.NewName, node)

	if _, ok := n.child[req.NewName]; ok {
		p_printf("file already exists at destination")
		return nil, errors.New("file already exists at destination")
	}

	n.child[req.NewName] = node
	node.attr.Nlink += 1

	return node, nil
}

//=============================================================================

//function passed onto FUSE for debugging
func logMsg(msg interface{}) {
	log.Printf("FUSE: %s\n", msg)
}

func main() {
	var flag int

	for {
		if flag = Getopt("dfm:"); flag == EOF {
			break
		}

		switch flag {
		case 'd':
			debug = !debug
		case 'f':
			fuse.Debug = logMsg
		case 'm':
			mountPoint = OptArg
		default:
			println("usage: main.go [-d | -f (fuse debug) | -m <mountpt>]", flag)
			os.Exit(1)
		}
	}
	p_printf("mounting on %q, debug %v", mountPoint, debug)

	if _, err := os.Stat(mountPoint); err != nil {
		os.Mkdir(mountPoint, 0755)
	}
	fuse.Unmount(mountPoint)
	conn, err := fuse.Mount(mountPoint, fuse.FSName("dssFS"), fuse.Subtype("project P1"),
		fuse.LocalVolume(), fuse.VolumeName("dssFS"))
	if err != nil {
		log.Fatal(err)
	}

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, os.Kill)
	go func() {
		<-ch
		defer conn.Close()
		fuse.Unmount(mountPoint)
		os.Exit(1)
	}()

	// root must be defined before here
	p_println("Creating root")
	root := createDir(1, "", os.ModeDir|0777)
	dfs = Dfs{root: root}
	dfs.nodeMap = make(map[uint64]*DNode)
	dfs.nodeMap[uint64(root.Inode())] = root
	p_println("root inode ", root.Inode())

	err = fs.Serve(conn, dfs)
	if err != nil {
		log.Fatal(err)
	}

	// check if the mount process has an error to report
	<-conn.Ready
	if err := conn.MountError; err != nil {
		log.Fatal(err)
	}
}
