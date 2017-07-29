package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strconv"
	"time"

	"github.com/russross/blackfriday"
)

type ServerData struct {
	index HTMLIndex
	theme HistoryTheme
}

func (sc *ServerData) Index(e *Environment) ([]byte, error) {
	// updating time
	sc.index.Time = time.Now().Format("2006-01-02 15:04:05")
	// gathering data
	for label, h := range e.History {
		sc.index.CSV = append(sc.index.CSV, HTMLLink{Name: label + ".csv", URL: filepath.Base(h.getPath(statsFile + csvExt))})

		statsNames := []struct {
			Name  string
			Label string
		}{
			{Name: "Buffer", Label: label + "_" + bufferStatsFile},
			{Name: "Upload", Label: label + "_" + uploadStatsFile},
			{Name: "Download", Label: label + "_" + downloadStatsFile},
			{Name: "Ratio", Label: label + "_" + ratioStatsFile},
			{Name: "Buffer/day", Label: label + "_" + bufferPerDayStatsFile},
			{Name: "Upload/day", Label: label + "_" + uploadPerDayStatsFile},
			{Name: "Download/day", Label: label + "_" + downloadPerDayStatsFile},
			{Name: "Ratio/day", Label: label + "_" + ratioPerDayStatsFile},
			{Name: "Snatches/day", Label: label + "_" + numberSnatchedPerDayFile},
			{Name: "Size Snatched/day", Label: label + "_" + sizeSnatchedPerDayFile},
		}
		// add graphs + links
		graphLinks := []HTMLLink{}
		graphs := []HTMLLink{}
		for _, s := range statsNames {
			graphLinks = append(graphLinks, HTMLLink{Name: s.Name, URL: "#" + s.Label})
			graphs = append(graphs, HTMLLink{Title: label + ": " + s.Name, Name: s.Label, URL: s.Label + svgExt})
		}
		// add previous stats (progress)
		var lastStats []*TrackerStats
		var lastStatsStrings [][]string
		if len(h.TrackerStats) < 25 {
			lastStats = h.TrackerStats
		} else {
			lastStats = h.TrackerStats[len(h.TrackerStats)-25 : len(h.TrackerStats)]
		}
		for i, s := range lastStats {
			if i == 0 {
				continue
			}
			lastStatsStrings = append(lastStatsStrings, s.ProgressParts(lastStats[i-1]))
		}
		// reversing
		for left, right := 0, len(lastStatsStrings)-1; left < right; left, right = left+1, right-1 {
			lastStatsStrings[left], lastStatsStrings[right] = lastStatsStrings[right], lastStatsStrings[left]
		}
		// TODO timestamps: first column for h.TrackerRecords.

		stats := HTMLStats{Name: label, TrackerStats: lastStatsStrings, Graphs: graphs, GraphLinks: graphLinks}
		sc.index.Stats = append(sc.index.Stats, stats)
	}
	return sc.index.ToHTML()
}

func (sc *ServerData) SaveIndex(e *Environment, file string) error {
	// building index
	data, err := sc.Index(e)
	if err != nil {
		return nil
	}
	// write to file
	return ioutil.WriteFile(file, data, 0644)
}

func (sc *ServerData) DownloadsList(e *Environment) ([]byte, error) {
	// TODO

	list := "<h1>Downloads</h1><ul>"
	for _, d := range e.Downloads.Downloads {
		list += fmt.Sprintf(`<li><a href="downloads/%d">%s</a></li>`, d.Index, d.RawShortString())
	}
	list += "</ul>"

	return []byte(list), nil

}

func (sc *ServerData) DownloadsInfo(e *Environment, id string) ([]byte, error) {
	// TODO

	// display individual download metadata
	downloadID, err := strconv.ParseUint(id, 10, 64)
	if err != nil {
		return []byte{}, errors.New("Error parsing download ID")
	}
	// find Download
	dl, err := e.Downloads.FindByID(downloadID)
	if err != nil {
		return []byte{}, errors.New("Error finding download ID " + id + " in db.")
	}

	response := []byte{}
	if dl.HasDescription {
		// TODO if more than 1 tracker, make things prettier
		for _, rinfo := range dl.ReleaseInfo {
			response = append(response, blackfriday.MarkdownCommon(rinfo)...)
		}
	} else {
		response = []byte(dl.RawShortString())
	}
	return response, nil

}
