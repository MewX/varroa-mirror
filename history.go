package main

import (
	"encoding/csv"
	"fmt"
	"os"
	"strconv"
	"time"
)

type History struct {
	Path string
	Releases []Release // TODO or simpler struct?
}

func (h *History) Add(r *Release, filter string) error {
	// preparing info
	timestamp := time.Now().Unix()
	// timestamp:filter:artist:title:year:size:type:source??
	info := []string{fmt.Sprintf("%d", timestamp), filter, r.artist, r.title, strconv.Itoa(r.year), strconv.FormatUint(r.size, 10), r.releaseType, r.source}
	// TODO append to h.Releases

	f, err := os.OpenFile(h.Path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	w := csv.NewWriter(f)
	if err := w.Write(info); err != nil {
		return err
	}
	w.Flush()
	return nil
}

func (h *History) Load(path string) error {
	h.Path = path
	// TODO called on startup


	return nil
}

func (h *History) HasDupe(r *Release) bool {
	// TODO check if r is already in history

	return false
}


func (h *History) StatsPerDay() ([]time.Time, []float64, []float64, error) {
	// TODO cut by day (timestamp == midnight)
	// TODO for each day, add number of release in a slice, total size in another
	now := time.Now().Unix()
	midnight := time.Unix(now, 0).Truncate(24*time.Hour)
	fmt.Println(now, midnight.Unix(), midnight)
	// add -24hours to go back 1 day
	fmt.Println(midnight.Add(time.Duration(-24)*time.Hour))

	return nil, nil, nil, nil
}

func (h *History) GenerateGraphs() error {
	// TODO generate graphs for snatches/day and amount of donwload/day


	return nil
}