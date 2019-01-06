package varroa

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"bazil.org/fuse"
	"bazil.org/fuse/fs"
	"github.com/djherbis/times"
	"github.com/pkg/errors"
	fs_ "gitlab.com/catastrophic/assistance/fs"
	"golang.org/x/net/context"
)

type FuseFile struct {
	fs               *FS
	releaseSubdir    string
	name             string
	trueRelativePath string
}

func (f *FuseFile) String() string {
	return fmt.Sprintf("FILE mount %s, trueRelativePath %s, release subdirectory %s, name %s", f.fs.mountPoint, f.trueRelativePath, f.releaseSubdir, f.name)
}

func (f *FuseFile) Attr(ctx context.Context, a *fuse.Attr) error {
	defer TimeTrack(time.Now(), fmt.Sprintf("FILE Attr %s.", f.String()))
	// get stat from the actual file
	fullPath := filepath.Join(f.fs.mountPoint, f.trueRelativePath, f.releaseSubdir, f.name)
	if !fs_.FileExists(fullPath) {
		return errors.New("Cannot find file " + fullPath)
	}
	// get stat
	var stat syscall.Stat_t
	if err := syscall.Stat(fullPath, &stat); err != nil {
		return errors.Wrap(err, "Error getting file status Stat_t "+fullPath)
	}
	a.Inode = stat.Ino
	a.Blocks = uint64(stat.Blocks)
	a.BlockSize = uint32(stat.Blksize)
	a.Size = uint64(stat.Size)
	a.Mode = 0555 // readonly
	// times are platform-specific
	t, err := times.Stat(fullPath)
	if err != nil {
		return errors.Wrap(err, "Error getting file times for "+fullPath)
	}
	a.Atime = t.AccessTime()
	a.Mtime = t.ModTime()
	if t.HasChangeTime() {
		a.Ctime = t.ChangeTime()
	}
	return nil
}

var _ = fs.NodeOpener(&FuseFile{})

func (f *FuseFile) Open(ctx context.Context, req *fuse.OpenRequest, resp *fuse.OpenResponse) (fs.Handle, error) {
	// logthis.Info(fmt.Sprintf("FILE Open %s.", f.String()), logthis.VERBOSESTEST)

	fullPath := filepath.Join(f.fs.mountPoint, f.trueRelativePath, f.releaseSubdir, f.name)
	if !fs_.FileExists(fullPath) {
		return nil, errors.New("File does not exist " + fullPath)
	}

	r, err := os.OpenFile(fullPath, int(req.Flags), 0555)
	if err != nil {
		return nil, errors.Wrap(err, "Error opening file "+fullPath)
	}
	return &FuseFileHandle{r: r, f: f}, nil
}

type FuseFileHandle struct {
	r *os.File
	f *FuseFile
}

var _ fs.Handle = (*FuseFileHandle)(nil)

var _ fs.HandleReleaser = (*FuseFileHandle)(nil)

func (fh *FuseFileHandle) Release(ctx context.Context, req *fuse.ReleaseRequest) error {
	// logthis.Info(fmt.Sprintf("FILE Release %s", fh.f.String()), logthis.VERBOSESTEST)
	if fh.r == nil {
		return fuse.EIO
	}
	return fh.r.Close()
}

var _ = fs.HandleReader(&FuseFileHandle{})

func (fh *FuseFileHandle) Read(ctx context.Context, req *fuse.ReadRequest, resp *fuse.ReadResponse) error {
	// logthis.Info(fmt.Sprintf("FILE Read %s", fh.f.String()), logthis.VERBOSESTEST)
	if fh.r == nil {
		return fuse.EIO
	}

	if _, err := fh.r.Seek(req.Offset, 0); err != nil {
		return err
	}
	buf := make([]byte, req.Size)
	n, err := fh.r.Read(buf)
	if err != nil && err != io.EOF {
		return errors.Wrap(err, "Error reading file "+fh.f.String())
	}
	resp.Data = buf[:n]
	return nil
}

var _ = fs.HandleFlusher(&FuseFileHandle{})

func (fh *FuseFileHandle) Flush(ctx context.Context, req *fuse.FlushRequest) error {
	// logthis.Info(fmt.Sprintf("Entered Flush with path: %s", fh.r.Name()), logthis.VERBOSESTEST)
	if fh.r == nil {
		return fuse.EIO
	}
	return fh.r.Sync()
}
