package varroa

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"bazil.org/fuse"
	"bazil.org/fuse/fs"
	"github.com/asdine/storm"
	"github.com/asdine/storm/q"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
)

var fuseCategories = []string{fuseArtistCategory, fuseTagsCategory, fuseLabelCategory, fuseYearCategory, fuseSourceCategory}

const (
	fuseArtistCategory = "artists"
	fuseTagsCategory   = "tags"
	fuseLabelCategory  = "record labels"
	fuseYearCategory   = "years"
	fuseSourceCategory = "source"
)

// FuseDir is a folder in the FUSE filesystem.
// Top directory == exposed categories, such as artists, tags.
// ex: artists/Radiohead/OK Computer/FILES
type FuseDir struct {
	fs       *FS
	category string
	label    string
	year     string
	tag      string
	artist   string
	source   string

	release       string
	releaseSubdir string
}

func (d *FuseDir) String() string {
	return fmt.Sprintf("DIR mount %s, category %s, tag %s, label %s, year %s, source %s, artist %s, release %s, release subdirectory %s", d.fs.mountPoint, d.category, d.tag, d.label, d.year, d.source, d.artist, d.release, d.releaseSubdir)
}

var _ = fs.Node(&FuseDir{})

func (d *FuseDir) Attr(ctx context.Context, a *fuse.Attr) error {
	defer TimeTrack(time.Now(), fmt.Sprintf("DIR ATTR %s", d.String()))
	fullPath := filepath.Join(d.fs.mountPoint, d.release, d.releaseSubdir)
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

func InSlice(field, v string) q.Matcher {
	return q.NewFieldMatcher(field, &sliceMatcher{value: v})
}

func (d *FuseDir) Lookup(ctx context.Context, name string) (fs.Node, error) {
	defer TimeTrack(time.Now(), "DIR LOOKUP "+name)

	// not music files, but files Dolphin tries to open nonetheless
	// returning directly saves a few DB searches doomed to fail
	if StringInSlice(name, []string{".hidden", ".directory"}) {
		return nil, fuse.EIO
	}

	// if top directory, show categories
	if d.category == "" {
		if StringInSlice(name, fuseCategories) {
			return &FuseDir{category: name, fs: d.fs}, nil
		} else {
			logThis.Info("Lookup unknown category: "+name, VERBOSEST)
			return nil, fuse.EIO
		}
	}

	// if we have a release, no need to look further, we can find what we need
	if d.release != "" {
		// find d.release and get its path
		// TODO: but d.release == folder name for now...
		var entry FuseEntry
		if err := d.fs.contents.DB.One("FolderName", d.release, &entry); err != nil {
			if err == storm.ErrNotFound {
				logThis.Info("Unkown release, could not find by path: "+d.release, VERBOSEST)
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
					return &FuseDir{category: d.category, tag: d.tag, label: d.label, year: d.year, artist: d.artist, source: d.source, release: d.release, releaseSubdir: filepath.Join(d.releaseSubdir, name), fs: d.fs}, nil
				}
				return &FuseFile{release: d.release, releaseSubdir: d.releaseSubdir, name: name, fs: d.fs}, nil
			}
		}
		logThis.Info("Unknown name among files "+d.releaseSubdir+"/"+name, VERBOSEST)
		return nil, fuse.EIO
	}

	// else, we have to filter things until we get to a release.
	// all these are extra layers that are above the inevitable "artists" level
	matcher := q.True()
	if d.category == fuseTagsCategory {
		if d.tag == "" {
			if !d.fs.contents.contains("Tags", name, true) {
				return nil, fuse.EIO
			}
			// we know there's at least 1 entry with this tag.
			return &FuseDir{category: d.category, tag: name, fs: d.fs}, nil
		}
		// if we have a tag, filter all releases with that tag
		matcher = q.And(matcher, InSlice("Tags", d.tag))
	}
	if d.category == fuseLabelCategory {
		if d.label == "" {
			if !d.fs.contents.contains("RecordLabel", name, false) {
				return nil, fuse.EIO
			}
			// we know there's at least 1 entry with this record label.
			return &FuseDir{category: d.category, label: name, fs: d.fs}, nil
		}
		// if we have a label, filter all releases with that record label
		matcher = q.And(matcher, q.Eq("RecordLabel", d.label))
	}
	if d.category == fuseYearCategory {
		if d.year == "" {
			if !d.fs.contents.contains("Year", name, false) {
				return nil, fuse.EIO
			}
			// we know there's at least 1 entry with this year.
			return &FuseDir{category: d.category, year: name, fs: d.fs}, nil
		}
		// if we have a year, filter all releases with that year
		matcher = q.And(matcher, q.Eq("Year", d.year))
	}
	if d.category == fuseSourceCategory {
		if d.source == "" {
			if !d.fs.contents.contains("Source", name, false) {
				return nil, fuse.EIO
			}
			// we know there's at least 1 entry with this source.
			return &FuseDir{category: d.category, source: name, fs: d.fs}, nil
		}
		// if we have a source, filter all releases with that source
		matcher = q.And(matcher, q.Eq("Source", d.source))
	}

	// if no artist is selected, return all artists for the filtered releases
	if d.artist == "" {
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
		return &FuseDir{category: d.category, tag: d.tag, label: d.label, year: d.year, artist: name, source: d.source, fs: d.fs}, nil
	}
	// if we have an artist, filter all releases with that artist
	matcher = q.And(matcher, InSlice("Artists", d.artist))

	// if we have an artist but not a release, return the filtered releases for this artist
	if d.release == "" {
		// name is a release name
		query := d.fs.contents.DB.Select(q.And(matcher, q.Eq("FolderName", name))).Limit(1)
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
		return &FuseDir{category: d.category, tag: d.tag, label: d.label, year: d.year, artist: d.artist, source: d.source, release: name, fs: d.fs}, nil
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
	for _, a := range allItems {
		allDirents = append(allDirents, fuse.Dirent{Name: a, Type: fuse.DT_Dir})
	}
	return allDirents, nil
}

// ReadDirAll returns directory entries for the FUSE filesystem
func (d *FuseDir) ReadDirAll(ctx context.Context) ([]fuse.Dirent, error) {
	defer TimeTrack(time.Now(), "DIR ReadDirAll "+d.String())

	// if root directory, return categories
	if d.category == "" {
		var categories []fuse.Dirent
		for _, c := range fuseCategories {
			categories = append(categories, fuse.Dirent{Name: c, Type: fuse.DT_Dir})
		}
		return categories, nil
	}

	// if we have a release, no need to look further, we can find what we need
	if d.release != "" {
		// find d.release and get its path
		// TODO: but d.release == folder name for now...
		var entry FuseEntry
		if err := d.fs.contents.DB.One("FolderName", d.release, &entry); err != nil {
			if err == storm.ErrNotFound {
				logThis.Info("Unkown release, could not find by path: "+d.release, VERBOSEST)
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
	switch d.category {
	case fuseTagsCategory:
		if d.tag == "" {
			return d.fuseDirs(matcher, "Tags")
		}
		matcher = q.And(matcher, InSlice("Tags", d.tag))
	case fuseLabelCategory:
		if d.label == "" {
			return d.fuseDirs(matcher, "RecordLabel")
		}
		matcher = q.And(matcher, q.Eq("RecordLabel", d.label))
	case fuseYearCategory:
		if d.year == "" {
			return d.fuseDirs(matcher, "Year")
		}
		matcher = q.And(matcher, q.Eq("Year", d.year))
	case fuseSourceCategory:
		if d.source == "" {
			return d.fuseDirs(matcher, "Source")
		}
		matcher = q.And(matcher, q.Eq("Source", d.source))
	}

	// the artist level always exists
	if d.artist == "" {
		return d.fuseDirs(matcher, "Artists")
	}
	matcher = q.And(matcher, InSlice("Artists", d.artist))

	// we have an artist but not a release, return all relevant releases
	if d.release == "" {
		return d.fuseDirs(matcher, "Folder")
	}
	return []fuse.Dirent{}, nil
}
