package main

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCommonInSlices(t *testing.T) {
	fmt.Println("+ Testing CommonInSlices...")
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
}
