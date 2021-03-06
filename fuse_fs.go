package varroa

import (
	"os"
	"syscall"

	"bazil.org/fuse"
	"bazil.org/fuse/fs"
	"github.com/pkg/errors"
	"gitlab.com/catastrophic/assistance/logthis"
	"golang.org/x/net/context"
)

type FS struct {
	mountPoint string
	contents   *FuseDB
}

var _ = fs.FS(&FS{})

func (f *FS) Root() (fs.Node, error) {
	return &FuseDir{fs: f}, nil
}

func (f *FS) Statfs(ctx context.Context, req *fuse.StatfsRequest, resp *fuse.StatfsResponse) error {
	wd, err := os.Getwd()
	if err != nil {
		return err
	}

	var stat syscall.Statfs_t
	if err := syscall.Statfs(wd, &stat); err != nil {
		return errors.Wrap(err, "Error getting stats call for "+wd)
	}
	resp.Blocks = stat.Blocks
	resp.Bfree = stat.Bfree
	resp.Bavail = stat.Bavail
	resp.Bsize = uint32(stat.Bsize)
	return nil
}

func FuseMount(path, mountpoint, dbPath string) error {
	// TODO checks

	// loading database
	db := &FuseDB{}
	if err := db.Open(dbPath); err != nil {
		return errors.Wrap(err, "Error loading db")
	}
	defer db.Close()

	go func() {
		// updating entries before serving
		if err := db.Scan(path); err != nil {
			logthis.Error(errors.Wrap(err, "Error scanning downloads"), logthis.NORMAL)
		}
	}()
	// TODO log how many entries

	// mounting
	mountOptions := []fuse.MountOption{
		fuse.FSName(FullNameAlt),
		fuse.Subtype(FullNameAlt + "FS"),
		fuse.VolumeName("Varroa Musica Library"),
	}
	c, err := fuse.Mount(mountpoint, mountOptions...)
	if err != nil {
		return errors.Wrap(err, "Error mounting fuse filesystem")
	}
	defer c.Close()
	filesys := &FS{mountPoint: path, contents: db}
	if err := fs.Serve(c, filesys); err != nil {
		return errors.Wrap(err, "Error serving fuse filesystem")
	}
	// check if the mount process has an error to report
	<-c.Ready
	if err := c.MountError; err != nil {
		return errors.Wrap(err, "Error with fuse mount")
	}
	return nil
}
