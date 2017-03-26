package main

import (
	"errors"
	"fmt"
	"strconv"
)

const (
	userStats     = "User: %s (%s) | "
	progress      = "Up: %s (%s) | Down: %s (%s) | Buffer: %s (%s) | Warning Buffer: %s (%s) | Ratio:  %.3f (%.3f)"
	firstProgress = "Up: %s | Down: %s | Buffer: %s | Warning Buffer: %s | Ratio: %.3f"
)

type TrackerStats struct {
	Username      string
	Class         string
	Up            uint64
	Down          uint64
	Buffer        uint64
	WarningBuffer uint64
	Ratio         float64
}

func (s *TrackerStats) Diff(previous *TrackerStats) (int64, int64, int64, int64, float64) {
	return int64(s.Up - previous.Up), int64(s.Down - previous.Down), int64(s.Buffer - previous.Buffer), int64(s.WarningBuffer - previous.WarningBuffer), s.Ratio - previous.Ratio
}

func (s *TrackerStats) Progress(previous *TrackerStats) string {
	if previous.Ratio == 0 {
		return s.String()
	}
	dup, ddown, dbuff, dwbuff, dratio := s.Diff(previous)
	return fmt.Sprintf(progress, readableUInt64(s.Up), readableInt64(dup), readableUInt64(s.Down), readableInt64(ddown), readableUInt64(s.Buffer), readableInt64(dbuff), readableUInt64(s.WarningBuffer), readableInt64(dwbuff), s.Ratio, dratio)
}

func (s *TrackerStats) IsProgressAcceptable(previous *TrackerStats, maxDecrease int) bool {
	if previous.Ratio == 0 {
		// first pass
		return true
	}
	_, _, bufferChange, _, _ := s.Diff(previous)
	if bufferChange > -int64(maxDecrease*1024*1024) {
		return true
	}
	logThis(fmt.Sprintf("Decrease: %d bytes, only %d allowed. Unacceptable.\n", bufferChange, maxDecrease*1024*1024), VERBOSE)
	return false
}

func (s *TrackerStats) String() string {
	return fmt.Sprintf(userStats, s.Username, s.Class) + fmt.Sprintf(firstProgress, readableUInt64(s.Up), readableUInt64(s.Down), readableUInt64(s.Buffer), readableUInt64(s.WarningBuffer), s.Ratio)
}

func (s *TrackerStats) ToSlice() []string {
	// up;down;ratio;buffer;warningBuffer
	return []string{strconv.FormatUint(s.Up, 10), strconv.FormatUint(s.Down, 10), strconv.FormatFloat(s.Ratio, 'f', -1, 64), strconv.FormatUint(s.Buffer, 10), strconv.FormatUint(s.WarningBuffer, 10)}
}

func (s *TrackerStats) FromSlice(slice []string) error {
	// slice contains timestamp, which is ignored
	if len(slice) != 6 {
		return errors.New("Incorrect entry, cannot load stats")
	}
	up, err := strconv.ParseUint(slice[1], 10, 64)
	if err != nil {
		return err
	}
	s.Up = up
	down, err := strconv.ParseUint(slice[2], 10, 64)
	if err != nil {
		return err
	}
	s.Down = down
	ratio, err := strconv.ParseFloat(slice[3], 64)
	if err != nil {
		return err
	}
	s.Ratio = ratio
	buffer, err := strconv.ParseUint(slice[4], 10, 64)
	if err != nil {
		return err
	}
	s.Buffer = buffer
	warningBuffer, err := strconv.ParseUint(slice[5], 10, 64)
	if err != nil {
		return err
	}
	s.WarningBuffer = warningBuffer
	return nil
}
