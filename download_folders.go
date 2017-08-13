package main

import (
	"strconv"
	"time"

	"github.com/pkg/errors"
)

type DownloadFolders []*DownloadFolder

// Add a *DownloadFolder
func (dfs *DownloadFolders) Add(folders ...*DownloadFolder) {
	for _, b := range folders {
		*dfs = append(*dfs, b)
	}
}

// filter DownloadFolders using a given function as a test for inclusion
func (dfs *DownloadFolders) filter(f func(*DownloadFolder) bool) DownloadFolders {
	filteredDownloadFolders := make(DownloadFolders, len(*dfs))
	for _, v := range *dfs {
		if f(v) {
			filteredDownloadFolders.Add(v)
		}
	}
	return filteredDownloadFolders
}

// findUnique DownloadFolder with a given function
func (dfs *DownloadFolders) findUnique(f func(*DownloadFolder) bool) (*DownloadFolder, error) {
	for i, v := range *dfs {
		if f(v) {
			return (*dfs)[i], nil
		}
	}
	return &DownloadFolder{}, errors.New("Not found")
}

func (dfs DownloadFolders) FilterArtist(artist string) DownloadFolders {
	return dfs.filter(func(dl *DownloadFolder) bool {
		if dl.HasInfo {
			for _, info := range dl.Metadata {
				if StringInSlice(artist, info.ArtistNames()) {
					return true
				}
			}
		}
		return false
	})
}

func (dfs DownloadFolders) FilterTag(tag string) DownloadFolders {
	return dfs.filter(func(dl *DownloadFolder) bool {
		if dl.HasInfo {
			for _, info := range dl.Metadata {
				if StringInSlice(tag, info.Release().Tags) {
					return true
				}
			}
		}
		return false
	})
}

func (dfs DownloadFolders) FilterRecordLabel(recordLabel string) DownloadFolders {
	return dfs.filter(func(dl *DownloadFolder) bool {
		if dl.HasInfo {
			for _, info := range dl.Metadata {
				if info.label == recordLabel {
					return true
				}
			}
		}
		return false
	})
}

func (dfs DownloadFolders) FilterYear(year string) DownloadFolders {
	iYear, err := strconv.Atoi(year)
	if err != nil {
		return DownloadFolders{}
	}
	return dfs.filter(func(dl *DownloadFolder) bool {
		if dl.HasInfo {
			for _, info := range dl.Metadata {
				if info.Release().Year == iYear {
					return true
				}
			}
		}
		return false
	})
}

func (dfs *DownloadFolders) FilterSortedState(state DownloadState) DownloadFolders {
	return dfs.filter(func(dl *DownloadFolder) bool { return dl.State == state })
}

func (dfs *DownloadFolders) FindByID(id uint64) (*DownloadFolder, error) {
	return dfs.findUnique(func(dl *DownloadFolder) bool { return dl.Index == id })
}

func (dfs *DownloadFolders) FindByPath(path string) (*DownloadFolder, error) {
	return dfs.findUnique(func(dl *DownloadFolder) bool { return dl.Path == path })
}

func (dfs DownloadFolders) AllArtists() []string {
	defer TimeTrack(time.Now(), "DFS ALLARTISTS")
	allArtists := []string{}
	for _, dl := range dfs {
		if dl.HasInfo {
			for _, info := range dl.Metadata {
				allArtists = append(allArtists, info.artistNames...)
			}
		}
	}
	return RemoveStringSliceDuplicates(allArtists)
}

func (dfs DownloadFolders) AllTags() []string {
	AllTags := []string{}
	for _, dl := range dfs {
		if dl.HasInfo {
			for _, info := range dl.Metadata {
				for _, a := range info.Release().Tags {
					if !StringInSlice(a, AllTags) {
						AllTags = append(AllTags, a)
					}
				}
			}
		}
	}
	return AllTags
}

func (dfs DownloadFolders) AllYears() []string {
	AllYears := []string{}
	for _, dl := range dfs {
		if dl.HasInfo {
			for _, info := range dl.Metadata {
				sYear := strconv.Itoa(info.Release().Year)
				if !StringInSlice(sYear, AllYears) {
					AllYears = append(AllYears, sYear)
				}
			}
		}
	}
	return AllYears
}

func (dfs DownloadFolders) AllRecordLabels() []string {
	AllLabels := []string{}
	for _, dl := range dfs {
		if dl.HasInfo {
			for _, info := range dl.Metadata {
				if !StringInSlice(info.label, AllLabels) {
					AllLabels = append(AllLabels, info.label)
				}
			}
		}
	}
	return AllLabels
}

func (dfs DownloadFolders) FolderNames() []string {
	folderNames := []string{}
	for _, dl := range dfs {
		folderNames = append(folderNames, dl.Path)
	}
	return folderNames
}
