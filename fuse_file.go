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

type File struct {
	fs            *FS
	category      string
	label         string
	year          string
	tag           string
	artist        string
	release       string
	releaseSubdir string
	name          string
}

func (f *File) String() string {
	return fmt.Sprintf("FILE mount %s, category %s, label %s, year %s, tag %s, artist %s, release %s, release subdirectory %s, name %s", f.fs.mountPoint, f.category, f.label, f.year, f.tag, f.artist, f.release, f.releaseSubdir, f.name)
}

func (f *File) Attr(ctx context.Context, a *fuse.Attr) error {
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

var _ = fs.NodeOpener(&File{})

func (f *File) Open(ctx context.Context, req *fuse.OpenRequest, resp *fuse.OpenResponse) (fs.Handle, error) {
	logThis.Info(fmt.Sprintf("FILE Open %s.", f.String()), VERBOSESTEST)

	fullPath := filepath.Join(f.fs.mountPoint, f.release, f.releaseSubdir, f.name)
	if !FileExists(fullPath) {
		return nil, errors.New("File does not exist " + fullPath)
	}

	r, err := os.OpenFile(fullPath, int(req.Flags), 0555)
	if err != nil {
		return nil, errors.Wrap(err, "Error opening file "+fullPath)
	}
	return &FileHandle{r: r, f: f}, nil
}

type FileHandle struct {
	r *os.File
	f *File
}

var _ fs.Handle = (*FileHandle)(nil)

var _ fs.HandleReleaser = (*FileHandle)(nil)

func (fh *FileHandle) Release(ctx context.Context, req *fuse.ReleaseRequest) error {
	logThis.Info(fmt.Sprintf("FILE Release %s", fh.f.String()), VERBOSESTEST)
	if fh.r == nil {
		return fuse.EIO
	}
	return fh.r.Close()
}

var _ = fs.HandleReader(&FileHandle{})

func (fh *FileHandle) Read(ctx context.Context, req *fuse.ReadRequest, resp *fuse.ReadResponse) error {
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

var _ = fs.HandleFlusher(&FileHandle{})

func (fh *FileHandle) Flush(ctx context.Context, req *fuse.FlushRequest) error {
	logThis.Info(fmt.Sprintf("Entered Flush with path: %s", fh.r.Name()), VERBOSESTEST)
	if fh.r == nil {
		return fuse.EIO
	}
	return fh.r.Sync()
}
