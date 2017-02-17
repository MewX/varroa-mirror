package main

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStats(t *testing.T) {
	fmt.Println("+ Testing Stats...")
	verify := assert.New(t)

	s1 := &Stats{}
	s2 := &Stats{Up: 1000 * 1024 * 1024, Down: 1000 * 1024 * 1024, Buffer: 1000 * 1024 * 1024, WarningBuffer: 1000 * 1024 * 1024, Ratio: float64(1.0)}
	s3 := &Stats{Up: 1050 * 1024 * 1024, Down: 2000 * 1024 * 1024, Buffer: 800 * 1024 * 1024, WarningBuffer: 1200 * 1024 * 1024, Ratio: float64(0.95)}
	s4 := &Stats{Up: 1551450749434, Down: 169522649052, Buffer: 1463583402983, WarningBuffer: 2416228600004, Ratio: 9.15187}
	s5 := &Stats{Up: 1551450749434, Down: 169522649052, Buffer: 1463382402983, WarningBuffer: 2416228600004, Ratio: 9.15187}
	s6 := &Stats{Up: 1551450749434, Down: 169522649052, Buffer: 1463563402983, WarningBuffer: 2416228600004, Ratio: 9.15187}
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
	verify.Equal(true, acceptable)
	acceptable = s3.IsProgressAcceptable(s2, 100)
	verify.Equal(false, acceptable)
	acceptable = s5.IsProgressAcceptable(s4, 100)
	verify.Equal(false, acceptable)
	acceptable = s6.IsProgressAcceptable(s4, 100)
	verify.Equal(true, acceptable)
}
