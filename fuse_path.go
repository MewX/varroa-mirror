package varroa

import (
	"errors"
	"fmt"

	"gitlab.com/catastrophic/assistance/logthis"
)

// list of valid FUSE path types
const (
	releasePath = iota
	artistPath
	sourcePath
	formatPath
	recordLabelPath
	tagPath
	yearPath
)

type fuseCategory struct {
	id         int
	field      string
	label      string
	sliceField bool
	validPath  []int
}

var fuseCategories = []fuseCategory{
	{id: artistPath, label: "artists", field: "Artists", sliceField: true, validPath: []int{artistPath, releasePath}},
	{id: tagPath, label: "tags", field: "Tags", sliceField: true, validPath: []int{tagPath, artistPath, releasePath}},
	{id: recordLabelPath, label: "record labels", field: "RecordLabel", validPath: []int{recordLabelPath, artistPath, releasePath}},
	{id: yearPath, label: "years", field: "Year", validPath: []int{yearPath, artistPath, releasePath}},
	{id: sourcePath, label: "source", field: "Source", validPath: []int{sourcePath, formatPath, artistPath, releasePath}},
	{id: formatPath, label: "format", field: "Format", validPath: []int{formatPath, sourcePath, artistPath, releasePath}},
	// {id: release, field: "FolderName", validPath: []int{}},
}

func fuseCategoryByLabel(label string) (fuseCategory, error) {
	for _, fc := range fuseCategories {
		if fc.label == label {
			return fc, nil
		}
	}
	return fuseCategory{}, errors.New("cannot find category for label " + label)
}

// FusePath holds all of the informations to generate a FUSE path.
type FusePath struct {
	category string
	label    string
	year     string
	tag      string
	artist   string
	source   string
	format   string
}

func (d *FusePath) String() string {
	return fmt.Sprintf("category %s, tag %s, label %s, year %s, source %s, format %s, artist %s", d.category, d.tag, d.label, d.year, d.source, d.format, d.artist)
}

func (d *FusePath) Category() string {
	category, err := fuseCategoryByLabel(d.category)
	if err != nil {
		logthis.Error(err, logthis.VERBOSEST)
		return ""
	}
	switch category.id {
	case artistPath:
		return d.artist
	case tagPath:
		return d.tag
	case recordLabelPath:
		return d.label
	case yearPath:
		return d.year
	case sourcePath:
		return d.source
	case formatPath:
		return d.format
	default:
		return ""
	}
}

func (d *FusePath) SetCategory(value string) error {
	category, err := fuseCategoryByLabel(d.category)
	if err != nil {
		logthis.Error(err, logthis.VERBOSEST)
		return err
	}
	switch category.id {
	case artistPath:
		d.artist = value
	case tagPath:
		d.tag = value
	case recordLabelPath:
		d.label = value
	case yearPath:
		d.year = value
	case sourcePath:
		d.source = value
	case formatPath:
		d.format = value
	default:
		return errors.New("category not found: " + value)
	}
	return nil
}
