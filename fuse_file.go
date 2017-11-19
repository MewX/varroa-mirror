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
	"github.com/pkg/errors"
	"golang.org/x/net/context"
)

type FuseFile struct {
	fs            *FS
	release       string
	releaseSubdir string
	name          string
}

func (f *FuseFile) String() string {
	return fmt.Sprintf("FILE mount %s, release %s, release subdirectory %s, name %s", f.fs.mountPoint, f.release, f.releaseSubdir, f.name)
}

func (f *FuseFile) Attr(ctx context.Context, a *fuse.Attr) error {
	defer TimeTrack(time.Now(), fmt.Sprintf("FILE Attr %s.", f.String()))
	// get stat from the actual file
	fullPath := filepath.Join(f.fs.mountPoint, f.release, f.releaseSubdir, f.name)
	if !FileExists(fullPath) {
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
	a.Atime = time.Unix(stat.Atim.Sec, stat.Atim.Nsec)
	a.Ctime = time.Unix(stat.Ctim.Sec, stat.Ctim.Nsec)
	a.Size = uint64(stat.Size)
	a.Mode = 0555 // readonly
	return nil
}

var _ = fs.NodeOpener(&FuseFile{})

func (f *FuseFile) Open(ctx context.Context, req *fuse.OpenRequest, resp *fuse.OpenResponse) (fs.Handle, error) {
	logThis.Info(fmt.Sprintf("FILE Open %s.", f.String()), VERBOSESTEST)

	fullPath := filepath.Join(f.fs.mountPoint, f.release, f.releaseSubdir, f.name)
	if !FileExists(fullPath) {
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
	logThis.Info(fmt.Sprintf("FILE Release %s", fh.f.String()), VERBOSESTEST)
	if fh.r == nil {
		return fuse.EIO
	}
	return fh.r.Close()
}

var _ = fs.HandleReader(&FuseFileHandle{})

func (fh *FuseFileHandle) Read(ctx context.Context, req *fuse.ReadRequest, resp *fuse.ReadResponse) error {
	logThis.Info(fmt.Sprintf("FILE Read %s", fh.f.String()), VERBOSESTEST)
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
	logThis.Info(fmt.Sprintf("Entered Flush with path: %s", fh.r.Name()), VERBOSESTEST)
	if fh.r == nil {
		return fuse.EIO
	}
	return fh.r.Sync()
}
