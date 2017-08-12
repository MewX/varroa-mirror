package main

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
	tag           string
	artist        string
	release       string
	releaseSubdir string
	name          string
}

func (f *File) String() string {
	return fmt.Sprintf("FILE mount %s, category %s, artist %s, release %s, release subdirectory %s, name %s", f.fs.mountPoint, f.category, f.artist, f.release, f.releaseSubdir, f.name)
}

func (f *File) Attr(ctx context.Context, a *fuse.Attr) error {
	fmt.Printf("FILE Attr %s.\n", f.String())
	// get stat from the actual file
	fullPath := filepath.Join(f.fs.mountPoint, f.release, f.releaseSubdir, f.name)
	if !FileExists(fullPath) {
		return errors.New("Cannot find file " + fullPath)
	}

	// open the file
	r, err := os.Open(fullPath)
	if err != nil {
		return errors.Wrap(err, "Error opening file "+fullPath)
	}
	defer r.Close()

	fi, err := r.Stat()
	if err != nil {
		return errors.Wrap(err, "Error getting file status "+fullPath)
	}

	a.Size = uint64(fi.Size())
	a.Mode = 0555 // readonly
	a.Atime = fi.ModTime()

	// TODO make only 1 call!

	var stat syscall.Stat_t
	if err := syscall.Stat(fullPath, &stat); err != nil {
		return errors.Wrap(err, "Error getting file status Stat_t "+fullPath)
	}
	a.Inode = stat.Ino
	a.Atime = time.Unix(stat.Atim.Sec, stat.Atim.Nsec)
	a.Ctime = time.Unix(stat.Ctim.Sec, stat.Ctim.Nsec)
	return nil
}

var _ = fs.NodeOpener(&File{})

func (f *File) Open(ctx context.Context, req *fuse.OpenRequest, resp *fuse.OpenResponse) (fs.Handle, error) {
	fmt.Printf("FILE Open %s.\n", f.String())

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
	if fh.r == nil {
		fmt.Printf("FILE Release: There is no file handler.\n")
		return fuse.EIO
	}
	fmt.Printf("FILE Release %s\n", fh.f.String())
	return fh.r.Close()
}

var _ = fs.HandleReader(&FileHandle{})

func (fh *FileHandle) Read(ctx context.Context, req *fuse.ReadRequest, resp *fuse.ReadResponse) error {
	fmt.Printf("FILE Read %s\n", fh.f.String())

	if fh.r == nil {
		fmt.Printf("There is no file handler.\n")
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
	if fh.f != nil {
		fmt.Printf("FILE Flush %s\n", fh.f.String())
	}
	if fh.r == nil {
		fmt.Printf("There is no file handler.\n")
		return fuse.EIO
	}
	fmt.Printf("Entered Flush with path: %s\n", fh.r.Name())
	return fh.r.Sync()
}
