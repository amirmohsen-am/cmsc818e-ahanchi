// memfs implements a simple in-memory file system.  v0.2
package main

/*
 Two main files are ../fuse.go and ../fs/serve.go
*/

import (
	"flag"
	"fmt"
	"log"
	"os"

	"bazil.org/fuse"
	"bazil.org/fuse/fs"
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

type DNode struct {
	nid   uint64
	name  string
	attr  fuse.Attr
	dirty bool
	kids  map[string]*DNode
	data  []uint8
}

var root *DNode

//  Compile error if DNode does not implement interface fs.Node, or if FS does not implement fs.FS
var _ fs.Node = (*DNode)(nil)
var _ fs.FS = (*Dfs)(nil)

var debug = false
var mountPoint = "dss"

//=============================================================================

func p_out(s string, args ...interface{}) {
	if !debug {
		return
	}
	fmt.Printf(s, args...)
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

func (n *DNode) Fsync(ctx context.Context, req *fuse.FsyncRequest) error {
	p_call("func (n *DNode) Fsync(ctx context.Context, req *fuse.FsyncRequest) error \n")
	p_out("FSYNC\n")
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

//=============================================================================

func main() {
	var flag int

	for {
		if flag = Getopt("dm:"); flag == EOF {
			break
		}

		switch flag {
		case 'd':
			debug = !debug
		case 'm':
			mountPoint = OptArg
		default:
			println("usage: main.go [-d | -m <mountpt>]", flag)
			os.Exit(1)
		}
	}
	p_out("mounting on %q, debug %v\n", mountPoint, debug)

	// root must be defined before here
	nodeMap[uint64(root.attr.Inode)] = root
	p_out("root inode %d", int(root.attr.Inode))

	if _, err := os.Stat(mountPoint); err != nil {
		os.Mkdir(mountPoint, 0755)
	}
	fuse.Unmount(mountPoint)
	c, err := fuse.Mount(mountPoint, fuse.FSName("dssFS"), fuse.Subtype("project P1"),
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

	err = fs.Serve(c, Dfs{})
	if err != nil {
		log.Fatal(err)
	}

	// check if the mount process has an error to report
	<-c.Ready
	if err := c.MountError; err != nil {
		log.Fatal(err)
	}
}
