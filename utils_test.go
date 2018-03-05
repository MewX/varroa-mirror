package varroa

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

type matchTestStructs struct {
	patterns  []string
	candidate string
	expected  bool
}

var matchTestData = []matchTestStructs{
	{[]string{"test"}, "test", true},
	{[]string{"one", "test", "vegetable"}, "test", true},
	{[]string{"one", "test", "vegetable"}, "tests", false},
	{[]string{"one", "r/test", "vegetable"}, "tests", true},
	{[]string{"r/^test.*$"}, "tests", true},
	{[]string{"test"}, "Test", false},
	{[]string{"r/[tT]est"}, "test", true},
	{[]string{"r/[tT]est"}, "Test", true},
	{[]string{"test"}, "greatest", false},
	{[]string{"r/test"}, "greatests", true},
	{[]string{"r/test$"}, "greatests", false},
	{[]string{"r/^test"}, "greatests", false},
}

func TestSliceHelpers(t *testing.T) {
	fmt.Println("+ Testing CommonInSlices + RemoveFromSlice + MatchInSlice...")
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

	for _, data := range matchTestData {
		result := MatchInSlice(data.candidate, data.patterns)
		check.Equal(data.expected, result)
	}
}

func TestSanitizeFolder(t *testing.T) {
	fmt.Println("+ Testing SanitizeFolder...")
	check := assert.New(t)

	check.Equal("hop", SanitizeFolder("////hop"))
	check.Equal("hop∕hop", SanitizeFolder("////hop∕hop"))
}
