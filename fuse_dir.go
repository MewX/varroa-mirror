package varroa

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"strconv"

	"bazil.org/fuse"
	"bazil.org/fuse/fs"
	"github.com/asdine/storm"
	"github.com/asdine/storm/q"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
)

// TODO: add categories
var fuseCategories = []string{fuseArtistCategory, fuseTagsCategory, fuseLabelCategory, fuseYearCategory}

const (
	fuseArtistCategory = "artists"
	fuseTagsCategory   = "tags"
	fuseLabelCategory  = "record labels"
	fuseYearCategory   = "years"
)

// Dir is a folder in the FUSE filesystem.
// Top directory == exposed categories, such as artists, tags.
// ex: artists/Radiohead/OK Computer/FILES
type Dir struct {
	fs            *FS
	category      string
	label         string
	year          string
	tag           string
	artist        string
	release       string
	releaseSubdir string
}

func (d *Dir) String() string {
	return fmt.Sprintf("DIR mount %s, category %s, tag %s, label %s, year %s, artist %s, release %s, release subdirectory %s", d.fs.mountPoint, d.category, d.tag, d.label, d.year, d.artist, d.release, d.releaseSubdir)
}

var _ = fs.Node(&Dir{})

func (d *Dir) Attr(ctx context.Context, a *fuse.Attr) error {
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

var _ = fs.NodeStringLookuper(&Dir{})

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

func (d *Dir) Lookup(ctx context.Context, name string) (fs.Node, error) {
	defer TimeTrack(time.Now(), "DIR LOOKUP "+name)

	// if top directory, show categories
	if d.category == "" {
		switch name {
		case fuseArtistCategory, fuseTagsCategory, fuseLabelCategory, fuseYearCategory:
			return &Dir{category: name, fs: d.fs}, nil
		default:
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
					return &Dir{category: d.category, tag: d.tag, label: d.label, year: d.year, artist: d.artist, release: d.release, releaseSubdir: filepath.Join(d.releaseSubdir, name), fs: d.fs}, nil
				} else {
					return &File{category: d.category, tag: d.tag, label: d.label, year: d.year, artist: d.artist, release: d.release, releaseSubdir: d.releaseSubdir, name: name, fs: d.fs}, nil
				}
			}
		}
		logThis.Info("Unknown name among files "+d.releaseSubdir+"/"+name, VERBOSEST)
		return nil, fuse.EIO
	}

	// else, we have to filter things until we get to a release.
	matcher := q.True()
	if d.category == fuseTagsCategory {
		// tags is an extra layer compared to "artists"
		if d.tag == "" {
			// name is a tag
			query := d.fs.contents.DB.Select(InSlice("Tags", name)).Limit(1)
			var entry FuseEntry
			if err := query.First(&entry); err != nil {
				if err == storm.ErrNotFound {
					logThis.Info("Unknown tag "+name, VERBOSEST)
					return nil, fuse.EIO
				} else {
					logThis.Error(err, VERBOSEST)
					return nil, fuse.EIO
				}
			}
			// we know there's at least 1 entry with this record label.
			return &Dir{category: d.category, tag: name, fs: d.fs}, nil
		} else {
			// if we have a tag, filter all releases with that tag
			matcher = q.And(matcher, InSlice("Tags", d.tag))
		}
	}
	if d.category == fuseLabelCategory {
		// labels is an extra layer compared to "artists"
		if d.label == "" {
			// name is a label
			query := d.fs.contents.DB.Select(q.Eq("RecordLabel", name)).Limit(1)
			var entry FuseEntry
			if err := query.First(&entry); err != nil {
				if err == storm.ErrNotFound {
					logThis.Info("Unknown record label "+name, VERBOSEST)
					return nil, fuse.EIO
				} else {
					logThis.Error(err, VERBOSEST)
					return nil, fuse.EIO
				}
			}
			// we know there's at least 1 entry with this record label.
			return &Dir{category: d.category, label: name, fs: d.fs}, nil
		} else {
			// if we have a label, filter all releases with that record label
			matcher = q.And(matcher, q.Eq("RecordLabel", d.label))
		}
	}
	if d.category == fuseYearCategory {
		// years is an extra layer compared to "artists"
		if d.year == "" {
			// name is a year
			query := d.fs.contents.DB.Select(q.Eq("Year", name)).Limit(1)
			var entry FuseEntry
			if err := query.First(&entry); err != nil {
				if err == storm.ErrNotFound {
					logThis.Info("Unknown year "+name, VERBOSEST)
					return nil, fuse.EIO
				} else {
					logThis.Error(err, VERBOSEST)
					return nil, fuse.EIO
				}
			}
			// we know there's at least 1 entry with this year.
			return &Dir{category: d.category, year: name, fs: d.fs}, nil
		} else {
			// if we have a label, filter all releases with that record label
			matcher = q.And(matcher, q.Eq("Year", d.year))
		}
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
			} else {
				logThis.Error(err, VERBOSEST)
				return nil, fuse.EIO
			}
		}
		// we know there's at least 1 entry with this artist.
		return &Dir{category: d.category, tag: d.tag, label: d.label, year: d.year, artist: name, fs: d.fs}, nil
	} else {
		// if we have an artist, filter all releases with that artist
		matcher = q.And(matcher, InSlice("Artists", d.artist))
	}

	// if we have an artist but not a release, return the filtered releases for this artist
	if d.release == "" {
		// name is a release name
		query := d.fs.contents.DB.Select(q.And(matcher, q.Eq("FolderName", name))).Limit(1)
		var entry FuseEntry
		if err := query.First(&entry); err != nil {
			if err == storm.ErrNotFound {
				logThis.Info("Unknown release "+name, VERBOSEST)
				return nil, fuse.EIO
			} else {
				logThis.Error(err, VERBOSEST)
				return nil, fuse.EIO
			}
		}
		// release was found
		return &Dir{category: d.category, tag: d.tag, label: d.label, year: d.year, artist: d.artist, release: name, fs: d.fs}, nil
	}
	logThis.Info("Error during lookup, nothing matched "+name, VERBOSEST)
	return nil, nil
}

var _ = fs.HandleReadDirAller(&Dir{})

func (d *Dir) ReadDirAll(ctx context.Context) ([]fuse.Dirent, error) {
	defer TimeTrack(time.Now(), "DIR ReadDirAll "+d.String())

	// if root directory, return categories
	if d.category == "" {
		categories := []fuse.Dirent{}
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
		actualFiles := []fuse.Dirent{}
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
	if d.category == fuseTagsCategory {
		// tags is an extra layer compared to "artists"
		if d.tag == "" {
			// return all tags as directories
			allTagsDirents := []fuse.Dirent{}
			// get all matching entries
			var allEntries []FuseEntry
			query := d.fs.contents.DB.Select(matcher)
			if err := query.Find(&allEntries); err != nil {
				logThis.Error(err, VERBOSEST)
				return allTagsDirents, err
			}
			// get all different years
			allTags := []string{}
			for _, e := range allEntries {
				allTags = append(allTags, e.Tags...)
			}
			for _, a := range RemoveStringSliceDuplicates(allTags) {
				allTagsDirents = append(allTagsDirents, fuse.Dirent{Name: a, Type: fuse.DT_Dir})
			}
			return allTagsDirents, nil
		} else {
			// if we have a tag, filter all releases with that tag
			matcher = q.And(matcher, InSlice("Tags", d.tag))
		}
	}
	if d.category == fuseLabelCategory {
		// labels is an extra layer compared to "artists"
		if d.label == "" {
			// return all labels as directories
			allLabelsDirents := []fuse.Dirent{}
			// get all matching entries
			var allEntries []FuseEntry
			query := d.fs.contents.DB.Select(matcher)
			if err := query.Find(&allEntries); err != nil {
				logThis.Error(err, VERBOSEST)
				return allLabelsDirents, err
			}
			// get all different years
			allLabels := []string{}
			for _, e := range allEntries {
				allLabels = append(allLabels, e.RecordLabel)
			}
			for _, a := range RemoveStringSliceDuplicates(allLabels) {
				allLabelsDirents = append(allLabelsDirents, fuse.Dirent{Name: a, Type: fuse.DT_Dir})
			}
			return allLabelsDirents, nil
		} else {
			// if we have a label, filter all releases with that record label
			matcher = q.And(matcher, q.Eq("RecordLabel", d.label))
		}
	}
	if d.category == fuseYearCategory {
		// years is an extra layer compared to "artists"
		if d.year == "" {
			// return all years as directories
			allYearsDirents := []fuse.Dirent{}
			// get all matching entries
			var allEntries []FuseEntry
			query := d.fs.contents.DB.Select(matcher)
			if err := query.Find(&allEntries); err != nil {
				logThis.Error(err, VERBOSEST)
				return allYearsDirents, err
			}
			// get all different years
			allYears := []string{}
			for _, e := range allEntries {
				allYears = append(allYears, strconv.Itoa(e.Year))
			}
			for _, a := range RemoveStringSliceDuplicates(allYears) {
				allYearsDirents = append(allYearsDirents, fuse.Dirent{Name: a, Type: fuse.DT_Dir})
			}
			return allYearsDirents, nil
		} else {
			// if we have a label, filter all releases with that record label
			matcher = q.And(matcher, q.Eq("Year", d.year))
		}
	}

	// if not artist set, return all artists from filtered releases
	if d.artist == "" {
		allArtistsDirents := []fuse.Dirent{}
		var allEntries []FuseEntry
		query := d.fs.contents.DB.Select(matcher)
		if err := query.Find(&allEntries); err != nil {
			logThis.Error(err, VERBOSEST)
			return allArtistsDirents, err
		}
		allArtists := []string{}
		for _, e := range allEntries {
			allArtists = append(allArtists, e.Artists...)
		}
		for _, a := range RemoveStringSliceDuplicates(allArtists) {
			allArtistsDirents = append(allArtistsDirents, fuse.Dirent{Name: a, Type: fuse.DT_Dir})
		}
		return allArtistsDirents, nil
	} else {
		// if we have an artist, filter all releases with that artist
		matcher = q.And(matcher, InSlice("Artists", d.artist))
	}

	// we have an artist but not a release, return all relevant releases
	if d.release == "" {
		allReleasesDirents := []fuse.Dirent{}
		// querying for all matches
		var allEntries []FuseEntry
		query := d.fs.contents.DB.Select(matcher)
		if err := query.Find(&allEntries); err != nil {
			logThis.Error(err, VERBOSEST)
			return allReleasesDirents, err
		}
		// getting the folder names
		for _, e := range allEntries {
			allReleasesDirents = append(allReleasesDirents, fuse.Dirent{Name: e.FolderName, Type: fuse.DT_Dir})
		}
		return allReleasesDirents, nil
	}
	return []fuse.Dirent{}, nil
}
