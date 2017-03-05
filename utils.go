package main

import (
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"path/filepath"
	"time"
)

const (
	NORMAL = iota
	VERBOSE
	VERBOSEST
)

func logThis(msg string, level int) {
	if conf.logLevel >= level {
		log.Print(msg)
	}
}

func startOfDay(t time.Time) time.Time {
	return t.Truncate(24*time.Hour)
}

func nextDay(t time.Time) time.Time {
	return t.Add(time.Duration(24)*time.Hour)
}

// StringInSlice checks if a string is in a []string, returns bool.
func StringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}

// IntInSlice checks if an int is in a []int, returns bool.
func IntInSlice(a int, list []int) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}

func checkErrors(errs ...error) error {
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

// DirectoryExists checks if a directory exists.
func DirectoryExists(path string) (res bool) {
	info, err := os.Stat(path)
	if err != nil {
		return
	}
	if info.IsDir() {
		return true
	}
	return
}

// AbsoluteFileExists checks if an absolute path is an existing file.
func AbsoluteFileExists(path string) (res bool) {
	info, err := os.Stat(path)
	if err != nil {
		return
	}
	if info.Mode().IsRegular() {
		return true
	}
	return
}

// FileExists checks if a path is valid and returns its absolute path
func FileExists(path string) (absolutePath string, err error) {
	currentDir, err := os.Getwd()
	if err != nil {
		return
	}
	var candidate string
	if filepath.IsAbs(path) {
		candidate = path
	} else {
		candidate = filepath.Join(currentDir, path)
	}

	if AbsoluteFileExists(candidate) {
		absolutePath = candidate
	} else {
		err = errors.New("File does not exist")
	}
	return
}

// CopyFile copies a file from src to dst. If src and dst files exist, and are
// the same, then return success. Otherwise, attempt to create a hard link
// between the two files. If that fail, copy the file contents from src to dst.
func CopyFile(src, dst string) (err error) {
	sfi, err := os.Stat(src)
	if err != nil {
		return
	}
	if !sfi.Mode().IsRegular() {
		// cannot copy non-regular files (e.g., directories,
		// symlinks, devices, etc.)
		return fmt.Errorf("CopyFile: non-regular source file %s (%q)", sfi.Name(), sfi.Mode().String())
	}
	dfi, err := os.Stat(dst)
	if err != nil {
		if !os.IsNotExist(err) {
			return
		}
	} else {
		if !(dfi.Mode().IsRegular()) {
			return fmt.Errorf("CopyFile: non-regular destination file %s (%q)", dfi.Name(), dfi.Mode().String())
		}
		if os.SameFile(sfi, dfi) {
			return
		}
	}
	err = copyFileContents(src, dst)
	return
}

// copyFileContents copies the contents of the file named src to the file named
// by dst. The file will be created if it does not already exist. If the
// destination file exists, all it's contents will be replaced by the contents
// of the source file.
func copyFileContents(src, dst string) (err error) {
	in, err := os.Open(src)
	if err != nil {
		return
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return
	}
	defer func() {
		cerr := out.Close()
		if err == nil {
			err = cerr
		}
	}()
	if _, err = io.Copy(out, in); err != nil {
		return
	}
	err = out.Sync()
	return
}

//-----------------------------------------------------------------------------

type ByteSize float64

const (
	_           = iota // ignore first value by assigning to blank identifier
	KB ByteSize = 1 << (10 * iota)
	MB
	GB
	TB
)

func (b ByteSize) String() string {
	switch {
	case b >= TB:
		return fmt.Sprintf("%.3fTB", b/TB)
	case b >= GB:
		return fmt.Sprintf("%.3fGB", b/GB)
	case b >= MB:
		return fmt.Sprintf("%.3fMB", b/MB)
	case b >= KB:
		return fmt.Sprintf("%.3fKB", b/KB)
	}
	return fmt.Sprintf("%.3fB", b)
}

func readableUInt64(a uint64) string {
	return ByteSize(float64(a)).String()
}
func readableInt64(a int64) string {
	if a >= 0 {
		return "+" + ByteSize(math.Abs(float64(a))).String()
	}
	return "-" + ByteSize(math.Abs(float64(a))).String()
}
