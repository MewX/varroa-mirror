package varroa

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"bazil.org/fuse"
	"bazil.org/fuse/fs"
	"github.com/asdine/storm"
	"github.com/asdine/storm/q"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
)

// FuseDir is a folder in the FUSE filesystem.
// Top directory == exposed categories, such as artists, tags.
// ex: artists/Radiohead/OK Computer/FILES
type FuseDir struct {
	fs               *FS
	path             FusePath
	trueRelativePath string
	release          string
	releaseSubdir    string
}

func (d *FuseDir) String() string {
	return fmt.Sprintf("DIR mount %s, path: %s, trueRelativePath %s, release %s, release subdirectory %s", d.fs.mountPoint, d.path.String(), d.trueRelativePath, d.release, d.releaseSubdir)
}

var _ = fs.Node(&FuseDir{})

func (d *FuseDir) Attr(ctx context.Context, a *fuse.Attr) error {
	defer TimeTrack(time.Now(), fmt.Sprintf("DIR ATTR %s", d.String()))
	fullPath := filepath.Join(d.fs.mountPoint, d.trueRelativePath, d.releaseSubdir)
	if !DirectoryExists(fullPath) {
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

var _ = fs.NodeStringLookuper(&FuseDir{})

func (d *FuseDir) Lookup(ctx context.Context, name string) (fs.Node, error) {
	defer TimeTrack(time.Now(), "DIR LOOKUP "+name)

	// not music files, but files Dolphin tries to open nonetheless
	// returning directly saves a few DB searches doomed to fail
	if StringInSlice(name, []string{".hidden", ".directory"}) {
		return nil, fuse.EIO
	}

	// if top directory, show categories
	if d.path.category == "" {
		_, err := fuseCategoryByLabel(name)
		if err != nil {
			logThis.Error(errors.Wrap(err, "Lookup unknown category: "+name), VERBOSEST)
			return nil, fuse.EIO
		}
		return &FuseDir{path: FusePath{category: name}, fs: d.fs}, nil
	}

	// if we have a release, no need to look further, we can find what we need
	if d.release != "" {
		// find d.release and get its path
		var entry FuseEntry
		if err := d.fs.contents.DB.One("FolderName", d.trueRelativePath, &entry); err != nil {
			if err == storm.ErrNotFound {
				logThis.Info("Unknown release, could not find by path: "+d.trueRelativePath, VERBOSEST)
			} else {
				logThis.Error(err, VERBOSEST)
			}
			return nil, fuse.ENOENT
		}
		folderPath := filepath.Join(d.fs.contents.Root, entry.FolderName, d.releaseSubdir)
		fileInfos, err := ioutil.ReadDir(folderPath)
		if err != nil {
			logThis.Info("Could not open path: "+d.release, VERBOSEST)
			return nil, fuse.ENOENT
		}
		for _, f := range fileInfos {
			if f.Name() == name {
				if f.IsDir() {
					return &FuseDir{path: FusePath{category: d.path.category, tag: d.path.tag, label: d.path.label, year: d.path.year, artist: d.path.artist, source: d.path.source, format: d.path.format}, trueRelativePath: d.trueRelativePath, release: d.release, releaseSubdir: filepath.Join(d.releaseSubdir, name), fs: d.fs}, nil
				}
				return &FuseFile{trueRelativePath: d.trueRelativePath, releaseSubdir: d.releaseSubdir, name: name, fs: d.fs}, nil
			}
		}
		logThis.Info("Unknown name among files "+d.releaseSubdir+"/"+name, VERBOSEST)
		return nil, fuse.EIO
	}

	// else, we have to filter things until we get to a release.
	// all these are extra layers that are above the inevitable "artists" level
	matcher := q.True()

	category, err := fuseCategoryByLabel(d.path.category)
	if err != nil {
		logThis.Error(err, VERBOSEST)
		return nil, fuse.EIO
	}
	if d.path.Category() == "" {
		if !d.fs.contents.contains(category.field, name, category.sliceField) {
			return nil, fuse.EIO
		}
		// we know there's at least 1 entry with this tag.
		p := FusePath{category: d.path.category}
		if err := p.SetCategory(name); err != nil {
			logThis.Error(err, VERBOSEST)
			return nil, fuse.EIO
		}
		return &FuseDir{path: p, fs: d.fs}, nil
	}
	// if we have a category, filter all releases with that category
	if category.sliceField {
		matcher = q.And(matcher, InSlice(category.field, d.path.Category()))
	} else {
		matcher = q.And(matcher, q.Eq(category.field, d.path.Category()))
	}

	// if no artist is selected, return all artists for the filtered releases
	if d.path.artist == "" {
		// name is an artist name, must be found among the already filtered releases
		query := d.fs.contents.DB.Select(q.And(matcher, InSlice("Artists", name))).Limit(1)
		var entry FuseEntry
		if err := query.First(&entry); err != nil {
			if err == storm.ErrNotFound {
				logThis.Info("Unknown artist "+name, VERBOSEST)
				return nil, fuse.EIO
			}
			logThis.Error(err, VERBOSEST)
			return nil, fuse.EIO
		}
		// we know there's at least 1 entry with this artist.
		return &FuseDir{path: FusePath{category: d.path.category, tag: d.path.tag, label: d.path.label, year: d.path.year, artist: name, source: d.path.source, format: d.path.format}, fs: d.fs}, nil
	}
	// if we have an artist, filter all releases with that artist
	matcher = q.And(matcher, InSlice("Artists", d.path.artist))

	// if we have an artist but not a release, return the filtered releases for this artist
	if d.release == "" {
		// name is a release name
		// NOTE: assumes that "name" is unique! will return the first hit. Only the complete FolderName should be unique in the db, however "name" will be the album folder, it should
		// be the part of the path that is unique. It might not be true, depending on library folder structure.
		query := d.fs.contents.DB.Select(q.And(matcher, HasSuffix("FolderName", name))).Limit(1)
		var entry FuseEntry
		if err := query.First(&entry); err != nil {
			if err == storm.ErrNotFound {
				logThis.Info("Unknown release "+name, VERBOSEST)
				return nil, fuse.EIO
			}
			logThis.Error(err, VERBOSEST)
			return nil, fuse.EIO
		}
		// release was found
		return &FuseDir{path: FusePath{category: d.path.category, tag: d.path.tag, label: d.path.label, year: d.path.year, artist: d.path.artist, source: d.path.source, format: d.path.format}, trueRelativePath: entry.FolderName, release: name, fs: d.fs}, nil
	}
	logThis.Info("Error during lookup, nothing matched "+name, VERBOSESTEST)
	return nil, nil
}

var _ = fs.HandleReadDirAller(&FuseDir{})

func (d *FuseDir) fuseDirs(matcher q.Matcher, field string) ([]fuse.Dirent, error) {
	var allDirents []fuse.Dirent
	allItems, err := d.fs.contents.uniqueEntries(matcher, field)
	if err != nil {
		return allDirents, err
	}
	allDirents = make([]fuse.Dirent, len(allItems))
	for _, a := range allItems {
		allDirents = append(allDirents, fuse.Dirent{Name: filepath.Base(a), Type: fuse.DT_Dir})
	}
	return allDirents, nil
}

// ReadDirAll returns directory entries for the FUSE filesystem
func (d *FuseDir) ReadDirAll(ctx context.Context) ([]fuse.Dirent, error) {
	defer TimeTrack(time.Now(), "DIR ReadDirAll "+d.String())

	// if root directory, return categories
	if d.path.category == "" {
		var categories []fuse.Dirent
		for _, c := range fuseCategories {
			categories = append(categories, fuse.Dirent{Name: c.label, Type: fuse.DT_Dir})
		}
		return categories, nil
	}

	// if we have a release, no need to look further, we can find what we need
	if d.release != "" {
		// find d.release and get its path
		// TODO: but d.release == folder name for now...
		var entry FuseEntry
		if err := d.fs.contents.DB.One("FolderName", d.trueRelativePath, &entry); err != nil {
			if err == storm.ErrNotFound {
				logThis.Info("Unknown release, could not find by path: "+d.trueRelativePath, VERBOSEST)
			} else {
				logThis.Error(err, VERBOSEST)
			}
			return []fuse.Dirent{}, fuse.ENOENT
		}
		folderPath := filepath.Join(d.fs.contents.Root, entry.FolderName, d.releaseSubdir)
		var actualFiles []fuse.Dirent
		contents, err := ioutil.ReadDir(folderPath)
		if err != nil {
			return []fuse.Dirent{}, fuse.ENOENT
		}
		for _, f := range contents {
			if f.IsDir() {
				actualFiles = append(actualFiles, fuse.Dirent{Name: f.Name(), Type: fuse.DT_Dir})
			} else {
				// TODO check it's a regular file, in case of symlinks or other?
				actualFiles = append(actualFiles, fuse.Dirent{Name: f.Name(), Type: fuse.DT_File})
			}
		}
		return actualFiles, nil
	}

	// else, we have to filter things until we get to a release.
	matcher := q.True()
	// all categories are a level above "artists"
	// either the category value is not set and we show the list of known values
	// or it is known and its value is used to make the matcher more precise
	category, err := fuseCategoryByLabel(d.path.category)
	if err != nil {
		logThis.Error(err, VERBOSEST)
		return []fuse.Dirent{}, fuse.ENOENT
	}
	if d.path.Category() == "" {
		return d.fuseDirs(matcher, category.field)
	}
	if category.sliceField {
		matcher = q.And(matcher, InSlice(category.field, d.path.Category()))
	} else {
		matcher = q.And(matcher, q.Eq(category.field, d.path.Category()))
	}

	// the artist level always exists
	if d.path.artist == "" {
		return d.fuseDirs(matcher, "Artists")
	}
	matcher = q.And(matcher, InSlice("Artists", d.path.artist))

	// we have an artist but not a release, return all relevant releases
	if d.release == "" {
		return d.fuseDirs(matcher, "FolderName")
	}
	return []fuse.Dirent{}, nil
}

// ------------
// custom db matchers

type sliceMatcher struct {
	value string
}

func (c *sliceMatcher) MatchField(v interface{}) (bool, error) {
	key, ok := v.([]string)
	if !ok {
		return false, nil
	}
	return StringInSlice(c.value, key), nil
}

// InSlice matches if one element of a []string is equal to the argument
func InSlice(field, v string) q.Matcher {
	return q.NewFieldMatcher(field, &sliceMatcher{value: v})
}

type suffixMatcher struct {
	value string
}

func (c *suffixMatcher) MatchField(v interface{}) (bool, error) {
	key, ok := v.(string)
	if !ok {
		return false, nil
	}
	return strings.HasSuffix(key, "/"+c.value), nil
}

// HasSuffix matches if the field value has the argument as suffix (subfolder)
func HasSuffix(field, v string) q.Matcher {
	return q.NewFieldMatcher(field, &suffixMatcher{value: v})
}
