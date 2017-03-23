package main

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/gorilla/mux"
)

const (
	webServerNotConfigured     = "No configuration found for the web server."
	errorServing               = "Error launching web interface: "
	errorWrongToken            = "Error receiving download order from https: wrong token"
	errorNoToken               = "Error receiving download order from https: no token"
	errorGeneratingCertificate = "Error generating self-signed certificate: "
	errorOpenSSL               = "openssl is not available on this system. "

	openssl        = "openssl"
	certificateKey = "key.pem"
	certificate    = "cert.pem"
)

var (
	provideCertificate         = fmt.Sprintf("You must provide your own self-signed certificate (%s & %s).", certificate, certificateKey)
	generateCertificateCommand = []string{"req", "-x509", "-nodes", "-days", "365", "-newkey", "rsa:2048", "-keyout", certificateKey, "-out", certificate, "-subj", "/C=IT/ST=Oregon/L=Moscow/O=varroa musica/OU=Org/CN=127.0.0.1"}
)

func webServer(tracker GazelleTracker) {
	if !conf.webserverConfigured() {
		logThis(webServerNotConfigured, NORMAL)
		return
	}

	// if not there yet, generate the self-signed certificate
	_, certificateKeyExists := FileExists(certificateKey)
	_, certificateExists := FileExists(certificate)
	if certificateExists == os.ErrNotExist || certificateKeyExists == os.ErrNotExist {
		// checking openssl is available
		_, err := exec.LookPath(openssl)
		if err != nil {
			logThis(errorOpenSSL+provideCertificate, NORMAL)
			return
		}
		// generate certificate
		if cmdOut, err := exec.Command(openssl, generateCertificateCommand...).Output(); err != nil {
			logThis(errorGeneratingCertificate+err.Error()+string(cmdOut), NORMAL)
			logThis(provideCertificate, NORMAL)
			return
		}
		// first connection will require manual approval since the certificate is self-signed, then things will work smoothly afterwards
	}

	rtr := mux.NewRouter()
	if conf.webServerAllowDownloads {
		// interface for remotely ordering downloads
		rtr.HandleFunc("/get/{id:[0-9]+}", func(w http.ResponseWriter, r *http.Request) {
			// checking token
			queryParameters := r.URL.Query()
			token, ok := queryParameters["token"]
			if !ok {
				logThis(errorNoToken, NORMAL)
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			if token[0] != conf.webServerToken {
				logThis(errorWrongToken, NORMAL)
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			// get torrent ID
			id := mux.Vars(r)["id"]
			release := &Release{torrentID: id, torrentURL: conf.url + "/torrents.php?action=download&id=" + id, filename: "remote-id" + id + ".torrent"}
			// get torrent info
			info, err := tracker.GetTorrentInfo(release.torrentID)
			if err != nil {
				logThis(errorCouldNotGetTorrentInfo, NORMAL)
				return // probably the ID does not exist
			}
			logThis("Downloading torrent #"+id, NORMAL)
			if _, err := tracker.Download(release); err != nil {
				logThis(errorDownloadingTorrent+release.torrentURL+" /  "+err.Error(), NORMAL)
			}
			// move to relevant watch directory
			if err := CopyFile(release.filename, filepath.Join(conf.defaultDestinationFolder, release.filename)); err != nil {
				logThis(errorCouldNotMoveTorrent+err.Error(), NORMAL)
				return
			}
			if err := os.Remove(release.filename); err != nil {
				logThis(fmt.Sprintf(errorRemovingTempFile, release.filename), VERBOSE)
			}
			// add to history ?
			// NOTE: or do we keep history for autosnatching only? would require filling in the Release struct from the info JSON
			// NOTE: this would allow sending release.ShortString to the notification

			// send notification
			if err := notification.Send("Snatched with web interface " + "torrent #" + id); err != nil {
				logThis(errorNotification+err.Error(), VERBOSE)
			}
			// save metadata once the download folder is created
			saveTrackerMetadata(info)
			w.WriteHeader(http.StatusOK)
		}).Methods("GET")
	}
	if conf.webServerServeStats {
		// serving static index.html in stats dir
		rtr.PathPrefix("/").Handler(http.FileServer(http.Dir(statsDir)))
	}
	http.Handle("/", rtr)
	// serve
	if err := http.ListenAndServeTLS(fmt.Sprintf(":%d", conf.webServerPort), certificate, certificateKey, nil); err != nil {
		logThis(errorServing+err.Error(), NORMAL)
	}
}
