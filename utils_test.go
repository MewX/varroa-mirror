package main

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSliceHelpers(t *testing.T) {
	fmt.Println("+ Testing CommonInSlices + RemoveFromSlice...")
	check := assert.New(t)

	a := []string{"1", "2", "3"}
	b := []string{"2", "3"}
	c := []string{"4", "8", "3"}
	d := []string{"9"}

	res1 := CommonInStringSlices(a, b)
	check.Equal([]string{"2", "3"}, res1)
	res2 := CommonInStringSlices(a, c)
	check.Equal([]string{"3"}, res2)
	res3 := CommonInStringSlices(a, d)
	check.Nil(res3)

	t1 := RemoveFromSlice("4", a)
	check.Equal(a, t1)
	t2 := RemoveFromSlice("1", a)
	check.Equal(b, t2)
}
