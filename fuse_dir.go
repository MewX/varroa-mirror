package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"bazil.org/fuse"
	"bazil.org/fuse/fs"
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
	defer TimeTrack(time.Now(), "DIR ATTR")
	logThis.Info(fmt.Sprintf("Attr %s", d.String()), VERBOSEST)
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

func (d *Dir) Lookup(ctx context.Context, name string) (fs.Node, error) {
	defer TimeTrack(time.Now(), "DIR LOOKUP")
	logThis.Info(fmt.Sprintf("Lookup name %s in  %s.", name, d.String()), VERBOSEST)

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
		dlFolder, err := d.fs.releases.FindByFolderName(d.release)
		if err != nil {
			logThis.Info("Unkown release, could not find by path: "+d.release, VERBOSEST)
			return nil, fuse.ENOENT
		}
		folderPath := filepath.Join(dlFolder.Root, dlFolder.Path, d.releaseSubdir)
		fileInfos, err := ioutil.ReadDir(folderPath)
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
	filteredReleases := d.fs.releases.Releases
	if d.category == fuseTagsCategory {
		// tags is an extra layer compared to "artists"
		if d.tag == "" {
			// name is a tag
			allTags := filteredReleases.AllTags()
			if StringInSlice(name, allTags) {
				return &Dir{category: d.category, tag: name, fs: d.fs}, nil
			} else {
				logThis.Info("Unknown tag "+name, VERBOSEST)
				return nil, fuse.EIO
			}
		} else {
			// if we have a tag, filter all releases with that tag
			filteredReleases = filteredReleases.FilterTag(d.tag)
		}
	}
	if d.category == fuseLabelCategory {
		// labels is an extra layer compared to "artists"
		if d.label == "" {
			// name is a label
			allLabels := filteredReleases.AllRecordLabels()
			if StringInSlice(name, allLabels) {
				return &Dir{category: d.category, label: name, fs: d.fs}, nil
			} else {
				logThis.Info("Unknown label "+name, VERBOSEST)
				return nil, fuse.EIO
			}
		} else {
			// if we have a label, filter all releases with that record label
			filteredReleases = filteredReleases.FilterRecordLabel(d.label)
		}
	}
	if d.category == fuseYearCategory {
		// years is an extra layer compared to "artists"
		if d.year == "" {
			// name is a label
			allYears := filteredReleases.AllYears()
			if StringInSlice(name, allYears) {
				return &Dir{category: d.category, year: name, fs: d.fs}, nil
			} else {
				logThis.Info("Unknown year "+name, VERBOSEST)
				return nil, fuse.EIO
			}
		} else {
			// if we have a label, filter all releases with that record label
			filteredReleases = filteredReleases.FilterYear(d.year)
		}
	}

	// if no artist is selected, return all artists for the filtered releases
	if d.artist == "" {
		// name is an artist name.
		// find name among all artists.
		allArtists := filteredReleases.AllArtists()
		if StringInSlice(name, allArtists) {
			return &Dir{category: d.category, tag: d.tag, label: d.label, year: d.year, artist: name, fs: d.fs}, nil
		} else {
			logThis.Info("Unknown artist "+name, VERBOSEST)
			return nil, fuse.EIO
		}
	}

	// if we have an artist but not a release, return the filtered releases for this artist
	if d.release == "" {
		// name is a release name
		// find release among releases of d.artist
		releasePaths := filteredReleases.FilterArtist(d.artist).FolderNames()
		if StringInSlice(name, releasePaths) {
			return &Dir{category: d.category, tag: d.tag, label: d.label, year: d.year, artist: d.artist, release: name, fs: d.fs}, nil
		} else {
			logThis.Info("Unknown release "+name, VERBOSEST)
			return nil, fuse.EIO
		}
	}
	return nil, nil
}

var _ = fs.HandleReadDirAller(&Dir{})

func (d *Dir) ReadDirAll(ctx context.Context) ([]fuse.Dirent, error) {
	defer TimeTrack(time.Now(), "DIR ReadDirAll")
	logThis.Info(fmt.Sprintf("ReadDirAll %s", d.String()), VERBOSEST)

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
		// return all files and folders inside the actual path as DT_Dir & DT_File.
		dlFolder, err := d.fs.releases.FindByFolderName(d.release)
		if err != nil {
			logThis.Info("Unkown release, could not find by path: "+d.release, VERBOSEST)
			return []fuse.Dirent{}, fuse.ENOENT
		}
		actualFiles := []fuse.Dirent{}
		contents, err := ioutil.ReadDir(filepath.Join(dlFolder.Root, dlFolder.Path, d.releaseSubdir))
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
	filteredReleases := d.fs.releases.Releases
	if d.category == fuseTagsCategory {
		// tags is an extra layer compared to "artists"
		if d.tag == "" {
			// return all tags as directories
			allTagsDirents := []fuse.Dirent{}
			allTags := filteredReleases.AllTags()
			for _, a := range allTags {
				allTagsDirents = append(allTagsDirents, fuse.Dirent{Name: a, Type: fuse.DT_Dir})
			}
			return allTagsDirents, nil
		} else {
			// if we have a tag, filter all releases with that tag
			filteredReleases = filteredReleases.FilterTag(d.tag)
		}
	}
	if d.category == fuseLabelCategory {
		// labels is an extra layer compared to "artists"
		if d.label == "" {
			// return all labels as directories
			allLabelsDirents := []fuse.Dirent{}
			allLabels := filteredReleases.AllRecordLabels()
			for _, a := range allLabels {
				allLabelsDirents = append(allLabelsDirents, fuse.Dirent{Name: a, Type: fuse.DT_Dir})
			}
			return allLabelsDirents, nil
		} else {
			// if we have a tag, filter all releases with that tag
			filteredReleases = filteredReleases.FilterRecordLabel(d.label)
		}
	}
	if d.category == fuseYearCategory {
		// years is an extra layer compared to "artists"
		if d.year == "" {
			// return all labels as directories
			allYearsDirents := []fuse.Dirent{}
			allYears := filteredReleases.AllYears()
			for _, a := range allYears {
				allYearsDirents = append(allYearsDirents, fuse.Dirent{Name: a, Type: fuse.DT_Dir})
			}
			return allYearsDirents, nil
		} else {
			// if we have a tag, filter all releases with that tag
			filteredReleases = filteredReleases.FilterYear(d.year)
		}
	}

	// if not artist set, return all artists from filtered releases
	if d.artist == "" {
		allArtistsDirents := []fuse.Dirent{}
		allArtists := filteredReleases.AllArtists()
		for _, a := range allArtists {
			allArtistsDirents = append(allArtistsDirents, fuse.Dirent{Name: a, Type: fuse.DT_Dir})
		}
		return allArtistsDirents, nil
	}

	// we have an artist but not a release, return all relevant releases
	if d.release == "" {
		allReleasesDirents := []fuse.Dirent{}
		releasePaths := filteredReleases.FilterArtist(d.artist).FolderNames()
		for _, a := range releasePaths {
			allReleasesDirents = append(allReleasesDirents, fuse.Dirent{Name: a, Type: fuse.DT_Dir})
		}
		return allReleasesDirents, nil
	}
	return []fuse.Dirent{}, nil
}
