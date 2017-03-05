package main

import (
	"encoding/csv"
	"fmt"
	"os"
	"time"
)

const (
	errorLoadingLine = "Error loading line %d of history file"
)

type History struct {
	Path     string
	Releases []*Release
}

func (h *History) Add(r *Release, filter string) error {
	// preparing info
	timestamp := time.Now().Unix()
	// timestamp;filter;artist;title;year;size;type;quality;source;format;tags
	info := []string{fmt.Sprintf("%d", timestamp), filter}
	info = append(info, r.ToSlice()...)

	// write to history file
	f, err := os.OpenFile(h.Path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	if err := w.Write(info); err != nil {
		return err
	}
	w.Flush()
	// add to in-memory slice
	h.Releases = append(h.Releases, r)
	return nil
}

func (h *History) Load(path string) error {
	h.Path = path
	// load history file
	f, err := os.OpenFile(h.Path, os.O_RDONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewReader(f)
	records, err := w.ReadAll()
	if err != nil {
		return err
	}
	// load releases from history to in-memory slice
	for i, record := range records {
		r := &Release{}
		if err := r.FromSlice(record); err != nil {
			logThis(fmt.Sprintf(errorLoadingLine, i), NORMAL)
		} else {
			h.Releases = append(h.Releases, r)
		}
	}
	return nil
}

func (h *History) HasDupe(r *Release) bool {
	// check if r is already in history
	for _, hr := range h.Releases {
		if r.IsDupe(hr) {
			return true
		}
	}
	return false
}

func (h *History) StatsPerDay() ([]time.Time, []float64, []float64, error) {
	// TODO cut by day (timestamp == midnight)
	// TODO for each day, add number of release in a slice, total size in another
	now := time.Now().Unix()
	midnight := time.Unix(now, 0).Truncate(24 * time.Hour)
	fmt.Println(now, midnight.Unix(), midnight)
	// add -24hours to go back 1 day
	fmt.Println(midnight.Add(time.Duration(-24) * time.Hour))

	return nil, nil, nil, nil
}

func (h *History) GenerateGraphs() error {
	// TODO generate graphs for snatches/day and amount of donwload/day

	return nil
}
