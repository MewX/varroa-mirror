package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGit(t *testing.T) {
	fmt.Println("\n --- Testing Git. ---")
	check := assert.New(t)

	testDir := "/tmp/testGit"
	dummyFile := filepath.Join(testDir, "file.txt")

	// create test dir
	if err := os.MkdirAll(testDir, 0777); err != nil {
		panic(err)
	}
	// create dummy file
	if err := ioutil.WriteFile(dummyFile, []byte("Nothing interesting."), 0777); err != nil {
		panic(err)
	}
	// remove everything once the test is over
	defer os.RemoveAll(testDir)

	// create struct
	git := NewGit(testDir, "user", "user@mail.com")
	check.NotNil(git)

	// test if in git dir, expect false
	exists := git.Exists()
	check.False(exists)

	// create git repo
	err := git.Init()
	check.Nil(err)

	// test if in git dir, expect true
	exists = git.Exists()
	check.True(exists)

	// add dummy file, expect nil
	err = git.Add(dummyFile)
	check.Nil(err)

	// add fake dummy file, expect err
	err = git.Add(dummyFile + "_")
	check.NotNil(err)

	// commit, expect nil
	err = git.Commit("commit")
	check.Nil(err)

	// lookup origin remote, expect false
	hasRemote := git.HasRemote("origin")
	check.False(hasRemote)

	// add remote origin, expect nil
	err = git.AddRemote("origin", "https://example/repository.git")
	check.Nil(err)

	// lookup origin remote, expect true
	hasRemote = git.HasRemote("origin")
	check.True(hasRemote)

	// check compress
	check.Nil(git.Compress())

	// how to test push?

}
