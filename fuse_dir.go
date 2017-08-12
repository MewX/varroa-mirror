package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"bazil.org/fuse"
	"bazil.org/fuse/fs"
	"golang.org/x/net/context"
)

// Dir is a folder in the FUSE filesystem.
// Top directory == exposed categories, such as artists, tags.
// ex: artists/Radiohead/OK Computer/FILES
type Dir struct {
	fs            *FS
	category      string
	tag           string
	artist        string
	release       string
	releaseSubdir string
}

func (d *Dir) String() string {
	return fmt.Sprintf("DIR mount %s, category %s, artist %s, release %s, release subdirectory %s", d.fs.mountPoint, d.category, d.artist, d.release, d.releaseSubdir)
}

var _ = fs.Node(&Dir{})

func (d *Dir) Attr(ctx context.Context, a *fuse.Attr) error {
	fmt.Printf("Attr %s\n", d.String())
	// read-only
	a.Mode = os.ModeDir | 0555
	a.Size = 4096
	return nil
}

var _ = fs.NodeStringLookuper(&Dir{})

func (d *Dir) Lookup(ctx context.Context, name string) (fs.Node, error) {
	fmt.Printf("Lookup name %s in  %s.\n", name, d.String())

	if d.category == "" {
		// top directory
		fmt.Println("Lookup top directory.")
		switch name {
		case "artists":
			return &Dir{category: "artists", fs: d.fs}, nil
		case "tags":
			return &Dir{category: "tags", fs: d.fs}, nil
		default:
			fmt.Println("Lookup unknown category: " + name)
			return nil, fuse.EIO
		}
	}

	// we have a category
	switch d.category {
	case "artists":
		if d.artist == "" {
			// name is an artist name.
			fmt.Println("Name is artist name!")
			// find name among all artists.
			allArtists := d.fs.releases.AllArtists()
			if StringInSlice(name, allArtists) {
				return &Dir{category: d.category, artist: name, fs: d.fs}, nil
			} else {
				fmt.Println("Unknown artist " + name)
				return nil, fuse.EIO
			}
		}
		// we also have an artist
		if d.release == "" {
			// name is a release name
			fmt.Println("Name is a release name!")
			// find release among releases of d.artist
			releasePaths := d.fs.releases.FilterByArtist(d.artist).FolderNames()
			if StringInSlice(name, releasePaths) {
				return &Dir{category: d.category, artist: d.artist, release: name, fs: d.fs}, nil
			} else {
				fmt.Println("Unknown release " + name)
				return nil, fuse.EIO
			}
		}
		// here we also have a release
		// name is an actual file or subfolder.
		fmt.Println("Name is a file or subfilder!")

		// find d.release and get its path
		dlFolder, err := d.fs.releases.FindByFolderName(d.release)
		if err != nil {
			fmt.Println("Unkown release, could not find by path: " + d.release)
			return nil, fuse.ENOENT
		}
		folderPath := filepath.Join(dlFolder.Root, dlFolder.Path, d.releaseSubdir)
		fileInfos, err := ioutil.ReadDir(folderPath)
		for _, f := range fileInfos {
			if f.Name() == name {
				if f.IsDir() {
					return &Dir{category: d.category, artist: d.artist, release: name, releaseSubdir: filepath.Join(d.releaseSubdir, name), fs: d.fs}, nil
				} else {
					return &File{category: d.category, artist: d.artist, release: d.release, releaseSubdir: d.releaseSubdir, name: name, fs: d.fs}, nil
				}
			}
		}
		fmt.Println("Lookup unknown name among files " + d.releaseSubdir + "/" + name)
		return nil, fuse.EIO

	case "tags":
		if d.tag == "" {
			//name is a tag
			fmt.Println("Name is a tag!")
			allTags := d.fs.releases.AllTags()
			if StringInSlice(name, allTags) {
				return &Dir{category: d.category, tag: name, fs: d.fs}, nil
			} else {
				fmt.Println("Unknown tag " + name)
				return nil, fuse.EIO
			}
		}

		// here we have a tag
		if d.artist == "" {
			// name is an artist name.
			fmt.Println("Name is artist name!")
			// find name among all artists.
			allArtists := d.fs.releases.FilterByTag(d.tag).AllArtists()
			if StringInSlice(name, allArtists) {
				return &Dir{category: d.category, tag: d.tag, artist: name, fs: d.fs}, nil
			} else {
				fmt.Println("Unknown artist " + name)
				return nil, fuse.EIO
			}
		}
		// we also have an artist
		if d.release == "" {
			// name is a release name
			fmt.Println("Name is a release name!")
			// find release among releases of d.artist
			releasePaths := d.fs.releases.FilterByTag(d.tag).FilterArtist(d.artist).FolderNames()
			if StringInSlice(name, releasePaths) {
				return &Dir{category: d.category, tag: d.tag, artist: d.artist, release: name, fs: d.fs}, nil
			} else {
				fmt.Println("Unknown release " + name)
				return nil, fuse.EIO
			}
		}
		// here we also have a release
		// name is an actual file or subfolder.
		fmt.Println("Name is a file or subfolder!")

		// find d.release and get its path
		dlFolder, err := d.fs.releases.FindByFolderName(d.release)
		if err != nil {
			fmt.Println("Unkown release, could not find by path: " + d.release)
			return nil, fuse.ENOENT
		}
		folderPath := filepath.Join(dlFolder.Root, dlFolder.Path, d.releaseSubdir)
		fileInfos, err := ioutil.ReadDir(folderPath)
		for _, f := range fileInfos {
			if f.Name() == name {
				if f.IsDir() {
					return &Dir{category: d.category, tag: d.tag, artist: d.artist, release: name, releaseSubdir: filepath.Join(d.releaseSubdir, name), fs: d.fs}, nil
				} else {
					return &File{category: d.category, tag: d.tag, artist: d.artist, release: d.release, releaseSubdir: d.releaseSubdir, name: name, fs: d.fs}, nil
				}
			}
		}
		fmt.Println("Lookup unknown name among files " + d.releaseSubdir + "/" + name)
		return nil, fuse.EIO

	default:
		fmt.Println("Lookup unknown category.")
		return nil, fuse.EIO
	}

	// TODO: add categories
	return nil, nil
}

// TODO: add categories
var categories = []fuse.Dirent{
	{Name: "artists", Type: fuse.DT_Dir},
	{Name: "tags", Type: fuse.DT_Dir},
}

var _ = fs.HandleReadDirAller(&Dir{})

func (d *Dir) ReadDirAll(ctx context.Context) ([]fuse.Dirent, error) {
	fmt.Printf("ReadDirAll %s\n", d.String())

	if d.category == "" {
		// top directory
		fmt.Println("ReadDirAll top directory.")
		return categories, nil
	}

	// we have a category
	switch d.category {
	case "artists":
		if d.artist == "" {
			fmt.Println("ReadDirAll artists directory.")
			// return all artists as directories
			allArtistsDirents := []fuse.Dirent{}

			// get all artist names: allArtists
			allArtists := d.fs.releases.AllArtists()
			for _, a := range allArtists {
				artistDiren := fuse.Dirent{Name: a, Type: fuse.DT_Dir}
				allArtistsDirents = append(allArtistsDirents, artistDiren)
			}
			return allArtistsDirents, nil
		}
		// we also have an artist
		if d.release == "" {
			fmt.Println("ReadDirAll releases from " + d.artist)
			// return all releases from d.artist as directories
			releasePaths := d.fs.releases.FilterByArtist(d.artist).FolderNames()
			allReleasesDirents := []fuse.Dirent{}
			for _, a := range releasePaths {
				allReleasesDirents = append(allReleasesDirents, fuse.Dirent{Name: a, Type: fuse.DT_Dir})
			}
			return allReleasesDirents, nil
		}

		fmt.Println("ReadDirAll files from  " + d.artist + " / " + d.release)
		// here we also have a release
		// return all files and folders inside the actual path as DT_Dir & DT_File.
		dlFolder, err := d.fs.releases.FindByFolderName(d.release)
		if err != nil {
			fmt.Println("Unkown release, could not find by path: " + d.release)
			return []fuse.Dirent{}, fuse.ENOENT
		}
		actualFiles := []fuse.Dirent{}
		contents, err := ioutil.ReadDir(filepath.Join(dlFolder.Root, dlFolder.Path))
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

	case "tags":
		if d.tag == "" {
			fmt.Println("ReadDirAll tags directory.")
			// return all tags as directories
			allTagsDirents := []fuse.Dirent{}

			// get all artist names: allArtists
			allTags := d.fs.releases.AllTags()
			for _, a := range allTags {
				tagDiren := fuse.Dirent{Name: a, Type: fuse.DT_Dir}
				allTagsDirents = append(allTagsDirents, tagDiren)
			}
			return allTagsDirents, nil
		}

		if d.artist == "" {
			fmt.Println("ReadDirAll artists directory.")
			// return all artists as directories
			allArtistsDirents := []fuse.Dirent{}

			// get all artist names: allArtists
			allArtists := d.fs.releases.FilterByTag(d.tag).AllArtists()
			for _, a := range allArtists {
				artistDiren := fuse.Dirent{Name: a, Type: fuse.DT_Dir}
				allArtistsDirents = append(allArtistsDirents, artistDiren)
			}
			return allArtistsDirents, nil
		}
		// we also have an artist
		if d.release == "" {
			fmt.Println("ReadDirAll releases from " + d.artist)
			// return all releases from d.artist as directories
			releasePaths := d.fs.releases.FilterByTag(d.tag).FilterArtist(d.artist).FolderNames()
			allReleasesDirents := []fuse.Dirent{}
			for _, a := range releasePaths {
				allReleasesDirents = append(allReleasesDirents, fuse.Dirent{Name: a, Type: fuse.DT_Dir})
			}
			return allReleasesDirents, nil
		}

		fmt.Println("ReadDirAll files from  " + d.artist + " / " + d.release)
		// here we also have a release
		// return all files and folders inside the actual path as DT_Dir & DT_File.
		dlFolder, err := d.fs.releases.FindByFolderName(d.release)
		if err != nil {
			fmt.Println("Unkown release, could not find by path: " + d.release)
			return []fuse.Dirent{}, fuse.ENOENT
		}
		actualFiles := []fuse.Dirent{}
		contents, err := ioutil.ReadDir(filepath.Join(dlFolder.Root, dlFolder.Path))
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

		// TODO add categories

	default:
		fmt.Println("ReadDirAll unknown category.")
		return []fuse.Dirent{}, fuse.ENOENT
	}

	return []fuse.Dirent{}, nil
}
