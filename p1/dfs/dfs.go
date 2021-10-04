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

var root *DNode

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
	p_println("Root, ", n)
	return dfs.root, nil
}

func (n *DNode) Attr(ctx context.Context, attr *fuse.Attr) error {
	p_println("Attr, ", n.Inode(), ", ", *attr)
	attr.Inode = n.attr.Inode
	attr.Mode = n.attr.Mode
	//TODO: set other attributes
	attr.Atime = n.attr.Atime
	attr.Mtime = n.attr.Mtime
	attr.Ctime = n.attr.Ctime
	return nil
}

func (n *DNode) Lookup(ctx context.Context, name string) (fs.Node, error) {
	p_printf("Lookup, %d, %s", n.Inode(), name)
	c, ok := n.child[name]
	if !ok {
		p_printf("%s not found", name)
		return nil, fuse.ENOENT
	}
	return c, nil
}

func (n *DNode) ReadDirAll(ctx context.Context) ([]fuse.Dirent, error) {
	p_printf("ReadDirAll, %d", n.Inode())
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
	p_printf("Getattr, %d", n.Inode())
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

// func (n *DNode) Setattr(ctx context.Context, req *fuse.SetattrRequest, resp *fuse.SetattrResponse) error {
// 	p_printf("Setattr, %d:", n.Inode())
// 	//p_println(attr)

// }

func createDir(nid uint64, name string, mode os.FileMode) *DNode {
	return &DNode{
		name:  name,
		attr:  fuse.Attr{Inode: nid, Mode: mode},
		child: make(map[string]*DNode),
	}
}

func (n *DNode) Mkdir(ctx context.Context, req *fuse.MkdirRequest) (fs.Node, error) {
	p_printf("Mkdir, %d, ", n.Inode())
	p_println(req)

	//TODO: check if directory?

	if _, ok := n.child[req.Name]; !ok {
		// nid:0 -> dynamic nid
		n.child[req.Name] = createDir(0, req.Name, req.Mode)

	} else {
		return nil, errors.New("Directory exists")
	}
	return n.child[req.Name], nil

}

//=============================================================================

//function passed onto FUSE for debugging
func logMsg(msg interface{}) {
	log.Printf("FUSE: %s\n", msg)
}

func main() {
	var flag int

	for {
		if flag = Getopt("dm:"); flag == EOF {
			break
		}

		switch flag {
		case 'd':
			debug = !debug
			fuse.Debug = logMsg
		case 'm':
			mountPoint = OptArg
		default:
			println("usage: main.go [-d | -m <mountpt>]", flag)
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
	dfs := Dfs{root: root}
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
