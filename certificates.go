package varroa

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/pkg/errors"
)

// Source: http://stackoverflow.com/questions/21297139/how-do-you-sign-certificate-signing-request-with-your-certification-authority/21340898#21340898

const (
	openssl         = "openssl"
	certificateKey  = "varroa_key.pem"
	certificate     = "varroa_cert.pem"
	certificatesDir = "certs"

	infoBackupScript    = "Generating certificates has failed. You can try to copy the certs directory and run generate_certificates.sh on your machine (requires openssl)."
	infoAddCertificates = "Certificates have been generated. If using Firefox, accept as security exception during the first connection. If using Chrome/Vivaldi on linux, close your browser, retrieve varroa_key.pem & varroa_cert.pem and run: certutil -d sql:$HOME/.pki/nssdb -A -t P -n varroa -i varroa_cert.pem"

	openSSLCA = `
HOME            = .
RANDFILE        = $ENV::HOME/.rnd

####################################################################
[ ca ]
default_ca  = CA_default
[ CA_default ]
default_days    = 1000          # how long to certify for
default_crl_days= 30            # how long before next CRL
default_md  = sha256            # use public key default MD
preserve    = no                # keep passed DN ordering
x509_extensions = ca_extensions # The extensions to add to the cert
email_in_dn = no                # Don't concat the email in the DN
copy_extensions = copy          # Required to copy SANs from CSR to cert
base_dir    = .
certificate = $base_dir/cacert.pem  # The CA certifcate
private_key = $base_dir/cakey.pem   # The CA private key
new_certs_dir   = $base_dir         # Location for new certs after signing
database    = $base_dir/index.txt   # Database index file
serial      = $base_dir/serial.txt  # The current serial number
unique_subject  = no            # Set to 'no' to allow creation of
                                # several certificates with same subject.
####################################################################
[ req ]
default_bits        = 4096
default_keyfile     = cakey.pem
distinguished_name  = ca_distinguished_name
x509_extensions     = ca_extensions
string_mask         = utf8only
####################################################################
[ ca_distinguished_name ]
countryName             = US
stateOrProvinceName     = Maryland
localityName            = Baltimore
organizationName        = Varroa Musica
organizationalUnitName  = Varroa
commonName              = %s
emailAddress            = varroa@musica.com
####################################################################
[ ca_extensions ]
subjectKeyIdentifier=hash
authorityKeyIdentifier=keyid:always, issuer
basicConstraints = critical, CA:true
keyUsage = keyCertSign, cRLSign
####################################################################
[ signing_policy ]
countryName    		= optional
stateOrProvinceName 	= optional
localityName        	= optional
organizationName    	= optional
organizationalUnitName  = optional
commonName      	= supplied
emailAddress        	= optional
####################################################################
[ signing_req ]
subjectKeyIdentifier=hash
authorityKeyIdentifier=keyid,issuer
basicConstraints = CA:FALSE
keyUsage = digitalSignature, keyEncipherment
`
	openSSLServer = `
HOME            = .
RANDFILE        = $ENV::HOME/.rnd

####################################################################
[ req ]
default_bits        = 2048
default_keyfile     = %s
distinguished_name  = server_distinguished_name
req_extensions      = server_req_extensions
string_mask         = utf8only
####################################################################
[ server_distinguished_name ]
countryName         	= US
stateOrProvinceName     = MD
localityName            = Baltimore
organizationName        = Varroa Musica
commonName          	= %s
emailAddress            = varroa@musica.com
####################################################################
[ server_req_extensions ]
subjectKeyIdentifier    = hash
basicConstraints        = CA:FALSE
keyUsage                = digitalSignature, keyEncipherment
subjectAltName          = @alternate_names
nsComment               = "OpenSSL Generated Certificate"
####################################################################
[ alternate_names ]
DNS.1       = %s
`
	openSSLshTemplate = `#!/bin/bash
openssl req -x509 -config %s -newkey rsa:4096 -sha256 -nodes -out cacert.pem -outform PEM -subj %s
openssl req -config %s -newkey rsa:2048 -sha256 -nodes -out servercert.csr -outform PEM -subj %s
openssl ca -batch -config %s -policy signing_policy -extensions signing_req -out %s -infiles servercert.csr
`
	openSSLCAConfFile       = "openssl-ca.cnf"
	openSSLServerConfFile   = "openssl-server.cnf"
	opensSLBackupScriptFile = "generate_certificates.sh"
)

var (
	provideCertificate    = fmt.Sprintf("You must provide your own self-signed certificate (%s & %s).", filepath.Join(certificatesDir, certificate), filepath.Join(certificatesDir, certificateKey))
	subjTemplate          = "/C=US/ST=MD/L=Baltimore/O=VarroaMusica/OU=varroa/CN=%s"
	generateCACommand     = []string{"req", "-x509", "-config", openSSLCAConfFile, "-newkey", "rsa:4096", "-sha256", "-nodes", "-out", "cacert.pem", "-outform", "PEM", "-subj"}
	generateServerCommand = []string{"req", "-config", openSSLServerConfFile, "-newkey", "rsa:2048", "-sha256", "-nodes", "-out", "servercert.csr", "-outform", "PEM", "-subj"}
	generateSignCommand   = []string{"ca", "-batch", "-config", openSSLCAConfFile, "-policy", "signing_policy", "-extensions", "signing_req", "-out", certificate, "-infiles", "servercert.csr"}
)

func generateCertificates(e *Environment) error {
	if !e.Config.webserverConfigured {
		return errors.New(webServerNotConfigured)
	}

	if err := os.MkdirAll(certificatesDir, 0777); err != nil {
		return errors.Wrap(err, errorCreatingCertDir)
	}
	// create the necessary files
	subj := fmt.Sprintf(subjTemplate, e.Config.WebServer.Hostname)
	if err := ioutil.WriteFile(filepath.Join(certificatesDir, "index.txt"), []byte(""), 0644); err != nil {
		return errors.Wrap(err, errorCreatingFile)
	}
	if err := ioutil.WriteFile(filepath.Join(certificatesDir, "serial.txt"), []byte("01"), 0644); err != nil {
		return errors.Wrap(err, errorCreatingFile)
	}
	if err := ioutil.WriteFile(filepath.Join(certificatesDir, openSSLCAConfFile), []byte(fmt.Sprintf(openSSLCA, e.Config.WebServer.Hostname)), 0644); err != nil {
		return errors.Wrap(err, errorCreatingFile)
	}
	if err := ioutil.WriteFile(filepath.Join(certificatesDir, openSSLServerConfFile), []byte(fmt.Sprintf(openSSLServer, certificateKey, e.Config.WebServer.Hostname, e.Config.WebServer.Hostname)), 0644); err != nil {
		return errors.Wrap(err, errorCreatingFile)
	}
	if err := ioutil.WriteFile(filepath.Join(certificatesDir, opensSLBackupScriptFile), []byte(fmt.Sprintf(openSSLshTemplate, openSSLCAConfFile, subj, openSSLServerConfFile, subj, openSSLCAConfFile, certificate)), 0644); err != nil {
		return errors.Wrap(err, errorCreatingFile)
	}

	// checking openssl is available
	if _, err := exec.LookPath(openssl); err != nil {
		return errors.New(errorOpenSSL + provideCertificate)
	}

	// generate certificate
	generateCACommand = append(generateCACommand, subj)
	cmdCA := exec.Command(openssl, generateCACommand...)
	cmdCA.Dir = certificatesDir
	if cmdOut, err := cmdCA.Output(); err != nil {
		return errors.Wrap(err, string(cmdOut))
	}
	// generate server certificate
	generateServerCommand = append(generateServerCommand, subj)
	cmdServer := exec.Command(openssl, generateServerCommand...)
	cmdServer.Dir = certificatesDir
	if cmdOut, err := cmdServer.Output(); err != nil {
		return errors.Wrap(err, string(cmdOut))
	}
	// signing server certificate
	cmdSign := exec.Command(openssl, generateSignCommand...)
	cmdSign.Dir = certificatesDir
	if cmdOut, err := cmdSign.Output(); err != nil {
		return errors.Wrap(err, string(cmdOut))
	}
	return nil
}
