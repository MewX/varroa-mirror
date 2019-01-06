package varroa

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStats(t *testing.T) {
	fmt.Println("+ Testing StatsEntry/Diff & IsProgressAcceptable...")

	// setting up
	verify := assert.New(t)

	// force config with dummy file
	_, err := NewConfig("test/test_statsnoautosnatch.yaml")
	verify.Nil(err)

	// test data
	s1 := &StatsEntry{Tracker: "blue"}
	s2 := &StatsEntry{Tracker: "blue", Up: 1000 * 1024 * 1024, Down: 1000 * 1024 * 1024, Ratio: float64(1.0)}
	s3 := &StatsEntry{Tracker: "blue", Up: 1050 * 1024 * 1024, Down: 2000 * 1024 * 1024, Ratio: float64(0.95)}
	s4 := &StatsEntry{Tracker: "blue", Up: 90 * 1024 * 1024, Down: 100 * 1024 * 1024, Ratio: 0.90}
	s5 := &StatsEntry{Tracker: "blue", Up: 90 * 1024 * 1024, Down: 145 * 1024 * 1024, Ratio: 0.62}
	s6 := &StatsEntry{Tracker: "blue", Up: 90 * 1024 * 1024, Down: 180 * 1024 * 1024, Ratio: 0.50}

	// check buffers
	buf2, wbuf2 := s2.getBufferValues()
	verify.Equal(int64(0), buf2)
	verify.Equal(int64(699050666), wbuf2)
	buf3, wbuf3 := s3.getBufferValues()
	verify.Equal(int64(-950*1024*1024), buf3)
	verify.Equal(int64(-262144000), wbuf3)

	// check first diff
	dup, ddown, dbuf, dwbuf, dratio := s2.Diff(s1)
	verify.Equal(int64(s2.Up), dup)
	verify.Equal(int64(s2.Down), ddown)
	verify.Equal(buf2, dbuf)
	verify.Equal(wbuf2, dwbuf)
	verify.Equal(s2.Ratio, dratio)
	// check diff
	dup, ddown, dbuf, dwbuf, dratio = s3.Diff(s2)
	verify.Equal(int64(50*1024*1024), dup)
	verify.Equal(int64(1000*1024*1024), ddown)
	verify.Equal(buf3-buf2, dbuf)
	verify.Equal(wbuf3-wbuf2, dwbuf)
	verify.InDelta(float64(-0.05), dratio, 0.001)

	// testing acceptability
	acceptable := s1.IsProgressAcceptable(s2, 100, 0.6)
	fmt.Println(s1.Progress(s2) + fmt.Sprintf(" | %v", acceptable))
	verify.False(acceptable)

	acceptable = s2.IsProgressAcceptable(s1, 100, 0.6)
	fmt.Println(s2.Progress(s1) + fmt.Sprintf(" | %v", acceptable))
	verify.True(acceptable)

	acceptable = s2.IsProgressAcceptable(s1, 100, 1.2)
	fmt.Println(s2.Progress(s1) + fmt.Sprintf(" | %v", acceptable))
	verify.False(acceptable)

	acceptable = s3.IsProgressAcceptable(s2, 100, 0.6)
	fmt.Println(s3.Progress(s2) + fmt.Sprintf(" | %v", acceptable))
	verify.False(acceptable)

	acceptable = s5.IsProgressAcceptable(s4, 5, 0.6)
	fmt.Println(s5.Progress(s4) + fmt.Sprintf(" | %v", acceptable))
	verify.False(acceptable)

	acceptable = s5.IsProgressAcceptable(s4, 100, 0.6)
	fmt.Println(s5.Progress(s4) + fmt.Sprintf(" | %v", acceptable))
	verify.True(acceptable)

	acceptable = s5.IsProgressAcceptable(s4, 100, 0.7)
	fmt.Println(s5.Progress(s4) + fmt.Sprintf(" | %v", acceptable))
	verify.False(acceptable)

	acceptable = s6.IsProgressAcceptable(s5, 5, 0.6)
	fmt.Println(s6.Progress(s5) + fmt.Sprintf(" | %v", acceptable))
	verify.False(acceptable)

	acceptable = s6.IsProgressAcceptable(s5, 100, 0.6)
	fmt.Println(s6.Progress(s5) + fmt.Sprintf(" | %v", acceptable))
	verify.False(acceptable)
}
