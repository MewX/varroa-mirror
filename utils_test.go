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
	{[]string{}, "greatests", false},
}

func TestSliceHelpers(t *testing.T) {
	fmt.Println("+ Testing MatchInSlice...")
	check := assert.New(t)

	a := []string{"1", "2", "3"}
	b := []string{"2", "3"}
	d := []string{"9"}

	// matchAllInSlice
	check.True(MatchAllInSlice(b, a))
	check.False(MatchAllInSlice(a, b))
	check.False(MatchAllInSlice(d, a))

	for _, data := range matchTestData {
		result := MatchInSlice(data.candidate, data.patterns)
		check.Equal(data.expected, result)
	}
}
