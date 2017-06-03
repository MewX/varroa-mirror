package main

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStats(t *testing.T) {
	fmt.Println("+ Testing TrackerStats...")

	// setting up
	verify := assert.New(t)
	env := NewEnvironment()
	logThis.env = env

	// test data
	s1 := &TrackerStats{}
	s2 := &TrackerStats{Up: 1000 * 1024 * 1024, Down: 1000 * 1024 * 1024, Buffer: 1000 * 1024 * 1024, WarningBuffer: 1000 * 1024 * 1024, Ratio: float64(1.0)}
	s3 := &TrackerStats{Up: 1050 * 1024 * 1024, Down: 2000 * 1024 * 1024, Buffer: 800 * 1024 * 1024, WarningBuffer: 1200 * 1024 * 1024, Ratio: float64(0.95)}
	s4 := &TrackerStats{Up: 1551450749434, Down: 169522649052, Buffer: 1463583402983, WarningBuffer: 2416228600004, Ratio: 9.15187}
	s5 := &TrackerStats{Up: 1551450749434, Down: 169522649052, Buffer: 1463382402983, WarningBuffer: 2416228600004, Ratio: 9.15187}
	s6 := &TrackerStats{Up: 1551450749434, Down: 169522649052, Buffer: 1463563402983, WarningBuffer: 2416228600004, Ratio: 9.15187}
	s7buffer := (90/0.95 - 100) * 1024 * 1024
	s7 := &TrackerStats{Up: 90 * 1024 * 1024, Down: 100 * 1024 * 1024, Buffer: int64(s7buffer), WarningBuffer: int64((90/0.60 - 100) * 1024 * 1024), Ratio: 0.90}
	s8buffer := (90/0.95 - 145) * 1024 * 1024
	s8 := &TrackerStats{Up: 90 * 1024 * 1024, Down: 145 * 1024 * 1024, Buffer: int64(s8buffer), WarningBuffer: int64((90/0.60 - 145) * 1024 * 1024), Ratio: 0.62}
	s9buffer := (90/0.95 - 180) * 1024 * 1024
	s9 := &TrackerStats{Up: 90 * 1024 * 1024, Down: 180 * 1024 * 1024, Buffer: int64(s9buffer), WarningBuffer: int64((90/0.60 - 180) * 1024 * 1024), Ratio: 0.50}

	// check first diff
	dup, ddown, dbuf, dwbuf, dratio := s2.Diff(s1)
	verify.Equal(int64(s2.Up), dup)
	verify.Equal(int64(s2.Down), ddown)
	verify.Equal(int64(s2.Buffer), dbuf)
	verify.Equal(int64(s2.WarningBuffer), dwbuf)
	verify.Equal(float64(s2.Ratio), dratio)
	// check diff
	dup, ddown, dbuf, dwbuf, dratio = s3.Diff(s2)
	verify.Equal(int64(50*1024*1024), dup)
	verify.Equal(int64(1000*1024*1024), ddown)
	verify.Equal(int64(-200*1024*1024), dbuf)
	verify.Equal(int64(200*1024*1024), dwbuf)
	verify.InDelta(float64(-0.05), dratio, 0.001)

	// testing acceptability
	acceptable := s2.IsProgressAcceptable(s1, 100)
	fmt.Println(s2.Progress(s1) + fmt.Sprintf(" | %v", acceptable))
	verify.True(acceptable)

	acceptable = s3.IsProgressAcceptable(s2, 100)
	fmt.Println(s3.Progress(s2) + fmt.Sprintf(" | %v", acceptable))
	verify.False(acceptable)

	acceptable = s5.IsProgressAcceptable(s4, 100)
	fmt.Println(s5.Progress(s4) + fmt.Sprintf(" | %v", acceptable))
	verify.False(acceptable)

	acceptable = s6.IsProgressAcceptable(s4, 100)
	fmt.Println(s6.Progress(s4) + fmt.Sprintf(" | %v", acceptable))
	verify.True(acceptable)

	acceptable = s2.IsProgressAcceptable(s1, 0)
	fmt.Println(s2.Progress(s1) + fmt.Sprintf(" | %v", acceptable))
	verify.True(acceptable)

	acceptable = s3.IsProgressAcceptable(s2, 10)
	fmt.Println(s3.Progress(s2) + fmt.Sprintf(" | %v", acceptable))
	verify.False(acceptable)

	acceptable = s5.IsProgressAcceptable(s4, 10)
	fmt.Println(s5.Progress(s4) + fmt.Sprintf(" | %v", acceptable))
	verify.False(acceptable)

	acceptable = s6.IsProgressAcceptable(s4, 20)
	fmt.Println(s6.Progress(s4) + fmt.Sprintf(" | %v", acceptable))
	verify.True(acceptable)

	acceptable = s1.IsProgressAcceptable(s2, 100)
	fmt.Println(s1.Progress(s2) + fmt.Sprintf(" | %v", acceptable))
	verify.False(acceptable)

	acceptable = s6.IsProgressAcceptable(s4, 5)
	fmt.Println(s6.Progress(s4) + fmt.Sprintf(" | %v", acceptable))
	verify.False(acceptable)

	acceptable = s8.IsProgressAcceptable(s7, 5)
	fmt.Println(s8.Progress(s7) + fmt.Sprintf(" | %v", acceptable))
	verify.False(acceptable)

	acceptable = s8.IsProgressAcceptable(s7, 100)
	fmt.Println(s8.Progress(s7) + fmt.Sprintf(" | %v", acceptable))
	verify.True(acceptable)

	acceptable = s9.IsProgressAcceptable(s8, 5)
	fmt.Println(s9.Progress(s8) + fmt.Sprintf(" | %v", acceptable))
	verify.False(acceptable)

	acceptable = s9.IsProgressAcceptable(s8, 100)
	fmt.Println(s9.Progress(s8) + fmt.Sprintf(" | %v", acceptable))
	verify.False(acceptable)
}
