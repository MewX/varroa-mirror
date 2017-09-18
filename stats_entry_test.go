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
	env := NewEnvironment()
	logThis.env = env

	// test data
	s1 := &StatsEntry{}
	s2 := &StatsEntry{Up: 1000 * 1024 * 1024, Down: 1000 * 1024 * 1024, Ratio: float64(1.0)}
	s3 := &StatsEntry{Up: 1050 * 1024 * 1024, Down: 2000 * 1024 * 1024, Ratio: float64(0.95)}
	s4 := &StatsEntry{Up: 1551450749434, Down: 169522649052, Ratio: 9.15187}
	s5 := &StatsEntry{Up: 1551450749434, Down: 169522649052, Ratio: 9.15187}
	s6 := &StatsEntry{Up: 1551450749434, Down: 169522649052, Ratio: 9.15187}
	s7 := &StatsEntry{Up: 90 * 1024 * 1024, Down: 100 * 1024 * 1024, Ratio: 0.90}
	s8 := &StatsEntry{Up: 90 * 1024 * 1024, Down: 145 * 1024 * 1024, Ratio: 0.62}
	s9 := &StatsEntry{Up: 90 * 1024 * 1024, Down: 180 * 1024 * 1024, Ratio: 0.50}

	// check first diff
	dup, ddown, dbuf, dwbuf, dratio := s2.Diff(s1)
	verify.Equal(int64(s2.Up), dup)
	verify.Equal(int64(s2.Down), ddown)
	verify.Equal(int64(1000*1024*1024), dbuf)
	verify.Equal(int64(1000*1024*1024), dwbuf)
	verify.Equal(float64(s2.Ratio), dratio)
	// check diff
	dup, ddown, dbuf, dwbuf, dratio = s3.Diff(s2)
	verify.Equal(int64(50*1024*1024), dup)
	verify.Equal(int64(1000*1024*1024), ddown)
	verify.Equal(int64(-200*1024*1024), dbuf)
	verify.Equal(int64(200*1024*1024), dwbuf)
	verify.InDelta(float64(-0.05), dratio, 0.001)

	// testing acceptability
	acceptable := s2.IsProgressAcceptable(s1, 100, 0.6)
	fmt.Println(s2.Progress(s1) + fmt.Sprintf(" | %v", acceptable))
	verify.True(acceptable)

	acceptable = s3.IsProgressAcceptable(s2, 100, 0.6)
	fmt.Println(s3.Progress(s2) + fmt.Sprintf(" | %v", acceptable))
	verify.False(acceptable)

	acceptable = s5.IsProgressAcceptable(s4, 100, 0.6)
	fmt.Println(s5.Progress(s4) + fmt.Sprintf(" | %v", acceptable))
	verify.False(acceptable)

	acceptable = s6.IsProgressAcceptable(s4, 100, 0.6)
	fmt.Println(s6.Progress(s4) + fmt.Sprintf(" | %v", acceptable))
	verify.True(acceptable)

	acceptable = s2.IsProgressAcceptable(s1, 0, 0.6)
	fmt.Println(s2.Progress(s1) + fmt.Sprintf(" | %v", acceptable))
	verify.True(acceptable)

	acceptable = s3.IsProgressAcceptable(s2, 10, 0.6)
	fmt.Println(s3.Progress(s2) + fmt.Sprintf(" | %v", acceptable))
	verify.False(acceptable)

	acceptable = s5.IsProgressAcceptable(s4, 10, 0.6)
	fmt.Println(s5.Progress(s4) + fmt.Sprintf(" | %v", acceptable))
	verify.False(acceptable)

	acceptable = s6.IsProgressAcceptable(s4, 20, 0.6)
	fmt.Println(s6.Progress(s4) + fmt.Sprintf(" | %v", acceptable))
	verify.True(acceptable)

	acceptable = s1.IsProgressAcceptable(s2, 100, 0.6)
	fmt.Println(s1.Progress(s2) + fmt.Sprintf(" | %v", acceptable))
	verify.False(acceptable)

	acceptable = s6.IsProgressAcceptable(s4, 5, 0.6)
	fmt.Println(s6.Progress(s4) + fmt.Sprintf(" | %v", acceptable))
	verify.False(acceptable)

	acceptable = s8.IsProgressAcceptable(s7, 5, 0.6)
	fmt.Println(s8.Progress(s7) + fmt.Sprintf(" | %v", acceptable))
	verify.False(acceptable)

	acceptable = s8.IsProgressAcceptable(s7, 100, 0.6)
	fmt.Println(s8.Progress(s7) + fmt.Sprintf(" | %v", acceptable))
	verify.True(acceptable)

	acceptable = s8.IsProgressAcceptable(s7, 100, 0.7)
	fmt.Println(s8.Progress(s7) + fmt.Sprintf(" | %v", acceptable))
	verify.False(acceptable)

	acceptable = s9.IsProgressAcceptable(s8, 5, 0.6)
	fmt.Println(s9.Progress(s8) + fmt.Sprintf(" | %v", acceptable))
	verify.False(acceptable)

	acceptable = s9.IsProgressAcceptable(s8, 100, 0.6)
	fmt.Println(s9.Progress(s8) + fmt.Sprintf(" | %v", acceptable))
	verify.False(acceptable)
}
