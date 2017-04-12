package main

import (
	"crypto/aes"
	"crypto/cipher"
	"errors"
	"fmt"
	"io/ioutil"
	"strings"

	"github.com/howeyc/gopass"
	yaml "gopkg.in/yaml.v2"
)

const (
	errorBadPassphrase        = "Error, passphrase must be 32bytes long."
	errorCanOnlyEncryptYAML   = "Error encrypting, input is not a .yaml file."
	errorCanOnlyDencryptENC   = "Error decrypting, input is not a .enc file."
	errorBadDecryptedFile     = "Decrypted file is not a valid YAML file (bad passphrase?): "
	errorReadingDecryptedFile = "Decrypted configuration file makes no sense."

	yamlExt      = ".yaml"
	encryptedExt = ".enc"
)

var commonIV = []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f}

func getPassphrase() (string, error) {
	// get passphrase, make sure it can fit into a 32b []byte
	fmt.Print("Passphrase: ")
	pass, err := gopass.GetPasswd()
	if err != nil {
		return "", err
	}
	if len(pass) > 32 {
		return "", errors.New("Passphrase must be at most 32 characters long.")
	}
	return string(pass), nil
}

func encrypt(path string, passphrase []byte) error {
	// config.yaml -> config.enc
	if len(passphrase) != 32 {
		return errors.New(errorBadPassphrase)
	}
	if !strings.HasSuffix(path, yamlExt) {
		return errors.New(errorCanOnlyEncryptYAML)
	}
	b, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}
	// create cipher
	c, err := aes.NewCipher(passphrase)
	if err != nil {
		return err
	}
	// Encrypted string
	cfb := cipher.NewCFBEncrypter(c, commonIV)
	encoded := make([]byte, len(b))
	cfb.XORKeyStream(encoded, b)

	// save to .enc
	return ioutil.WriteFile(strings.TrimSuffix(path, yamlExt)+encryptedExt, encoded, 0644)
}

func decrypt(path string, passphrase []byte) ([]byte, error) {
	if len(passphrase) != 32 {
		return []byte{}, errors.New(errorBadPassphrase)
	}
	if !strings.HasSuffix(path, encryptedExt) {
		return []byte{}, errors.New(errorCanOnlyDencryptENC)
	}
	b, err := ioutil.ReadFile(path)
	if err != nil {
		return []byte{}, err
	}
	// create cipher
	c, err := aes.NewCipher(passphrase)
	if err != nil {
		return []byte{}, err
	}
	cfbdec := cipher.NewCFBDecrypter(c, commonIV)
	decoded := make([]byte, len(b))
	cfbdec.XORKeyStream(decoded, b)

	// check it's valid YAML?
	var config QuickCheckConfig
	if err := yaml.Unmarshal(decoded, &config); err != nil {
		return []byte{}, errors.New(errorBadDecryptedFile + err.Error())
	}
	if config.Tracker.Password == "" || config.Tracker.User == "" || config.Tracker.URL == "" {
		return []byte{}, errors.New(errorReadingDecryptedFile)
	}
	return decoded, nil
}

func decryptAndSave(path string, passphrase []byte) error {
	decoded, err := decrypt(path, passphrase)
	if err != nil {
		return err
	}
	// save to .yaml
	return ioutil.WriteFile(strings.TrimSuffix(path, encryptedExt)+yamlExt, decoded, 0644)
}

// QuickCheckConfig helps checking the decoded bytes look like a valid configuration file
type QuickCheckConfig struct {
	Tracker struct {
		URL      string
		User     string
		Password string
	}
}

func encryptConfigurationFile() error {
	passphrase, err := getPassphrase()
	if err != nil {
		return err
	}
	copy(configPassphrase[:], passphrase)
	return encrypt(defaultConfigurationFile, configPassphrase)
}

func decryptConfigurationFile() error {
	passphrase, err := getPassphrase()
	if err != nil {
		return err
	}
	copy(configPassphrase[:], passphrase)
	encryptedConfigurationFile := strings.TrimSuffix(defaultConfigurationFile, yamlExt) + encryptedExt
	return decryptAndSave(encryptedConfigurationFile, configPassphrase)
}
