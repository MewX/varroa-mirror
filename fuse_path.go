package varroa

import (
	"errors"
	"fmt"
)

// list of valid FUSE path types
const (
	release = iota
	artist
	source
	format
	recordLabel
	tag
	year
)

type fuseCategory struct {
	id         int
	field      string
	label      string
	sliceField bool
	validPath  []int
}

var fuseCategories = []fuseCategory{
	{id: artist, label: "artists", field: "Artists", sliceField: true, validPath: []int{artist, release}},
	{id: tag, label: "tags", field: "Tags", sliceField: true, validPath: []int{tag, artist, release}},
	{id: recordLabel, label: "record labels", field: "RecordLabel", validPath: []int{recordLabel, artist, release}},
	{id: year, label: "years", field: "Year", validPath: []int{year, artist, release}},
	{id: source, label: "source", field: "Source", validPath: []int{source, format, artist, release}},
	// {id: format, label: "format", field: "Format", validPath: []int{format, source, artist, release}},
	// {id: release, field: "FolderName", validPath: []int{}},
}

func fuseCategoryByLabel(label string) (fuseCategory, error) {
	for _, fc := range fuseCategories {
		if fc.label == label {
			return fc, nil
		}
	}
	return fuseCategory{}, errors.New("cannot find category for label" + label)
}

type FusePath struct {
	category string
	label    string
	year     string
	tag      string
	artist   string
	source   string
}

func (d *FusePath) String() string {
	return fmt.Sprintf("category %s, tag %s, label %s, year %s, source %s, artist %s", d.category, d.tag, d.label, d.year, d.source, d.artist)
}

func (d *FusePath) Category() string {
	category, err := fuseCategoryByLabel(d.category)
	if err != nil {
		logThis.Error(err, VERBOSEST)
		return ""
	}
	switch category.id {
	case artist:
		return d.artist
	case tag:
		return d.tag
	case recordLabel:
		return d.label
	case year:
		return d.year
	case source:
		return d.source
	default:
		return ""
	}
}

func (d *FusePath) SetCategory(value string) error {
	category, err := fuseCategoryByLabel(d.category)
	if err != nil {
		logThis.Error(err, VERBOSEST)
		return err
	}
	switch category.id {
	case artist:
		d.artist = value
	case tag:
		d.tag = value
	case recordLabel:
		d.label = value
	case year:
		d.year = value
	case source:
		d.source = value
	default:
		return errors.New("category not found: " + value)
	}
	return nil
}
