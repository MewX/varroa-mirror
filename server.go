package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
)

const (
	webServerNotConfigured     = "No configuration found for the web server."
	webServerShutDown          = " - Web server has closed."
	webServerUp                = " - Starting web server."
	errorServing               = "Error launching web interface: "
	errorWrongToken            = "Error receiving download order from https: wrong token"
	errorNoToken               = "Error receiving download order from https: no token"
	errorGeneratingCertificate = "Error generating self-signed certificate: "
	errorOpenSSL               = "openssl is not available on this system. "
	errorNoID                  = "Error retreiving torrent ID"

	openssl        = "openssl"
	certificateKey = "key.pem"
	certificate    = "cert.pem"
)

var (
	provideCertificate         = fmt.Sprintf("You must provide your own self-signed certificate (%s & %s).", certificate, certificateKey)
	generateCertificateCommand = []string{"req", "-x509", "-nodes", "-days", "365", "-newkey", "rsa:2048", "-keyout", certificateKey, "-out", certificate, "-subj", "/C=IT/ST=Oregon/L=Moscow/O=varroa musica/OU=Org/CN=127.0.0.1"}
)

// TODO add token + command
// TODO check token!!!!
type JSONMessage struct {
	Status  int
	Message string
}

func webServer() {
	if !conf.webserverConfigured() {
		logThis(webServerNotConfigured, NORMAL)
		return
	}

	/*
		TODO: make this work
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
	*/

	rtr := mux.NewRouter()
	if conf.webServerAllowDownloads {
		getTorrent := func(w http.ResponseWriter, r *http.Request) {
			queryParameters := r.URL.Query()
			// get torrent ID
			id, ok := mux.Vars(r)["id"]
			if !ok {
				// if it's not in URL, try to get from query parameters
				queryID, ok2 := queryParameters["id"]
				if !ok2 {
					logThis(errorNoID, NORMAL)
					w.WriteHeader(http.StatusUnauthorized) // TODO find better status code?
					return
				}
				id = queryID[0]
			}
			// checking token
			token, ok := queryParameters["token"]
			if !ok {
				// try to get token from "pass" parameter instead
				token, ok = queryParameters["pass"]
				if !ok {
					logThis(errorNoToken, NORMAL)
					w.WriteHeader(http.StatusUnauthorized)
					return
				}
			}
			if token[0] != conf.webServerToken {
				logThis(errorWrongToken, NORMAL)
				w.WriteHeader(http.StatusUnauthorized)
				return
			}

			// get torrent info
			info, err := tracker.GetTorrentInfo(id)
			if err != nil {
				logThis(errorCouldNotGetTorrentInfo, NORMAL)
				return // probably the ID does not exist
			}
			release := info.Release()
			if release == nil {
				logThis("Error parsing Torrent Info", NORMAL)
				release = &Release{TorrentID: id}
			}
			release.torrentURL = conf.url + "/torrents.php?action=download&id=" + id
			release.TorrentFile = "remote-id" + id + ".torrent"

			logThis("Web server: downloading torrent "+release.ShortString(), NORMAL)
			if _, err := tracker.Download(release); err != nil {
				logThis(errorDownloadingTorrent+release.torrentURL+" /  "+err.Error(), NORMAL)
			}
			// move to relevant watch directory
			if err := CopyFile(release.TorrentFile, filepath.Join(conf.defaultDestinationFolder, release.TorrentFile)); err != nil {
				logThis(errorCouldNotMoveTorrent+err.Error(), NORMAL)
				return
			}
			if err := os.Remove(release.TorrentFile); err != nil {
				logThis(fmt.Sprintf(errorRemovingTempFile, release.TorrentFile), VERBOSE)
			}
			// add to history
			if err := history.SnatchHistory.Add(release, "remote"); err != nil {
				logThis(errorAddingToHistory, NORMAL)
			}
			// send notification
			if err := notification.Send("Snatched with web interface: " + release.ShortString()); err != nil {
				logThis(errorNotification+err.Error(), VERBOSE)
			}
			// write response
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("<html><head><script>t = null;function moveMe(){t = setTimeout(\"self.close()\",5000);}</script></head><body onload=\"moveMe()\">Successfully downloaded torrent: " + release.ShortString() + "</body></html>"))
			// save metadata once the download folder is created
			saveTrackerMetadata(info)
		}
		upgrader := websocket.Upgrader{
			// allows connection to websocket from anywhere
			CheckOrigin: func(r *http.Request) bool { return true },
		}
		socket := func(w http.ResponseWriter, r *http.Request) {
			c, err := upgrader.Upgrade(w, r, nil)
			if err != nil {
				log.Print("upgrade:", err)
				return
			}
			defer c.Close()
			for {
				incoming := JSONMessage{}
				err := c.ReadJSON(&incoming)
				if err != nil {
					if websocket.IsCloseError(err, websocket.CloseGoingAway) {
						break
					}
					logThis("Error reading from websocket: "+err.Error(), NORMAL)
					break
				}
				log.Printf("recv: %s %d", incoming.Message, incoming.Status)

				hello := JSONMessage{Status: 1, Message: "hello"}
				if err := c.WriteJSON(hello); err != nil {
					fmt.Println("NOPE")
				}
			}
		}
		// interface for remotely ordering downloads
		rtr.HandleFunc("/get/{id:[0-9]+}", getTorrent).Methods("GET")
		rtr.HandleFunc("/dl.pywa", getTorrent).Methods("GET")
		rtr.HandleFunc("/ws", socket)
	}
	if conf.webServerServeStats {
		// serving static index.html in stats dir
		rtr.PathPrefix("/").Handler(http.FileServer(http.Dir(statsDir)))
	}

	// serve
	logThis(webServerUp, NORMAL)
	server = &http.Server{Addr: fmt.Sprintf(":%d", conf.webServerPort), Handler: rtr}
	//if err := server.ListenAndServeTLS(certificate, certificateKey); err != nil {
	if err := server.ListenAndServe(); err != nil {
		if err == http.ErrServerClosed {
			logThis(webServerShutDown, NORMAL)
		} else {
			logThis(errorServing+err.Error(), NORMAL)
		}
	}
}
