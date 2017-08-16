package varroa

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCrypto(t *testing.T) {
	fmt.Println("\n --- Testing Crypto. ---")
	check := assert.New(t)

	testDir := "test"
	testFilename := "test_crypto"
	testYAML := filepath.Join(testDir, testFilename+yamlExt)
	testENC := filepath.Join(testDir, testFilename+encryptedExt)
	env := &Environment{}
	var passphrase []byte
	passphrase = make([]byte, 32)
	copy(passphrase[:], "passphrase")

	check.Nil(env.Config.Load(testYAML))

	// 1. encrypt

	// bad passphrase
	err := encryptAndSave(testYAML, []byte("tooshort"))
	check.NotNil(err)
	// not yaml
	err = encryptAndSave(testYAML+"--", passphrase)
	check.NotNil(err)
	// normal
	err = encryptAndSave(testYAML, passphrase)
	check.Nil(err)
	check.True(FileExists(testENC))
	defer os.Remove(testENC)

	// preparing for decrypt test
	os.Rename(testYAML, testYAML+"_original")
	// at the end, restore original file
	defer os.Rename(testYAML+"_original", testYAML)

	// 2. decrypt

	// bad passphrase
	_, err = decrypt(testENC, []byte("tooshort"))
	check.NotNil(err)
	// not yaml
	_, err = decrypt(testENC+"--", passphrase)
	check.NotNil(err)
	// decrypt
	bOut, err := decrypt(testENC, passphrase)
	check.Nil(err)
	// check decoded bytes can be loaded as Config
	c := &Config{}
	err = c.LoadFromBytes(bOut)
	check.Nil(err)
	check.Equal("https://something.com", c.Trackers[0].URL)
	check.Equal("i_am", c.Trackers[0].User)
	check.Equal("a_test", c.Trackers[0].Password)

	// 3. decrypt and save

	err = decryptAndSave(testENC, passphrase)
	check.Nil(err)
	check.True(FileExists(testYAML))

	// check contents are the same
	bOut, err = ioutil.ReadFile(testYAML)
	check.Nil(err)
	bIn, err := ioutil.ReadFile(testYAML + "_original")
	check.Nil(err)
	check.Equal(bIn, bOut)

	// remove generated file
	os.Remove(testYAML)
}
