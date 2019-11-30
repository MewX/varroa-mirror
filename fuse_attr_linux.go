package varroa

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"bazil.org/fuse"
	"github.com/pkg/errors"
	fs_ "gitlab.com/catastrophic/assistance/fs"
	"golang.org/x/net/context"
)

func (d *FuseDir) Attr(ctx context.Context, a *fuse.Attr) error {
	defer TimeTrack(time.Now(), fmt.Sprintf("DIR ATTR %s", d.String()))
	fullPath := filepath.Join(d.fs.mountPoint, d.trueRelativePath, d.releaseSubdir)
	if !fs_.DirExists(fullPath) {
		return errors.New("Cannot find directory " + fullPath)
	}
	// get stat
	var stat syscall.Stat_t
	if err := syscall.Stat(fullPath, &stat); err != nil {
		return errors.Wrap(err, "Error getting dir status Stat_t "+fullPath)
	}
	a.Inode = stat.Ino
	a.Blocks = uint64(stat.Blocks)
	a.BlockSize = uint32(stat.Blksize)
	a.Atime = time.Unix(stat.Atim.Sec, stat.Atim.Nsec)
	a.Ctime = time.Unix(stat.Ctim.Sec, stat.Ctim.Nsec)
	a.Size = uint64(stat.Size)
	a.Mode = os.ModeDir | 0555 // readonly

	return nil
}
