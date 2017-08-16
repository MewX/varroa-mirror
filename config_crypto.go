package varroa

import (
	"crypto/aes"
	"crypto/cipher"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/howeyc/gopass"
	"github.com/pkg/errors"
	daemon "github.com/sevlyar/go-daemon"
	yaml "gopkg.in/yaml.v2"
)

var commonIV = []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f}

func GetPassphrase() (string, error) {
	// get passphrase, make sure it can fit into a 32b []byte
	fmt.Print("Passphrase: ")
	pass, err := gopass.GetPasswd()
	if err != nil {
		return "", err
	}
	if len(pass) > 32 {
		return "", errors.New(errorBadPassphrase)
	}
	return string(pass), nil
}

// SavePassphraseForDaemon saves the encrypted configuration file passphrase to env if necessary.
// In the daemon, retrieves that passphrase.
func SavePassphraseForDaemon() ([]byte, error) {
	var passphrase string
	var err error
	if !daemon.WasReborn() {
		// if necessary, ask for passphrase and add to env
		passphrase, err = GetPassphrase()
		if err != nil {
			return []byte{}, errors.Wrap(err, errorGettingPassphrase)
		}
		// saving to env for the daemon to pick up later
		if err := os.Setenv(envPassphrase, passphrase); err != nil {
			return []byte{}, errors.Wrap(err, errorSettingEnv)
		}
	} else {
		// getting passphrase from env if necessary
		passphrase = os.Getenv(envPassphrase)
	}
	if passphrase == "" {
		return []byte{}, errors.New(errorPassphraseNotFound)
	}
	b := make([]byte, 32)
	copy(b[:], passphrase)
	return b, nil
}

func encryptAndSave(path string, passphrase []byte) error {
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
	var config Config
	if err := yaml.Unmarshal(decoded, &config); err != nil {
		return []byte{}, errors.Wrap(err, errorBadDecryptedFile)
	}
	if config.Trackers[0].Password == "" || config.Trackers[0].User == "" || config.Trackers[0].URL == "" {
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
