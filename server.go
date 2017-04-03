package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/goji/httpauth"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
)

const (
	webServerNotConfigured = "No configuration found for the web server."
	webServerShutDown      = " - Web server has closed."
	webServerUpHTTP        = " - Starting http web server."
	webServerUpHTTPS       = " - Starting https web server."
	webServersUp           = " - Web server(s) started."
	errorServing           = "Error launching web interface: "
	errorWrongToken        = "Error receiving download order from https: wrong token"
	errorNoToken           = "Error receiving download order from https: no token"
	errorNoID              = "Error retreiving torrent ID"

	downloadCommand  = "get"
	handshakeCommand = "hello"
	statsCommand     = "stats"

	errorUnknownCommand          = "Error: unknown websocket command: "
	errorIncomingWebSocketJSON   = "Error parsing websocket input: "
	errorIncorrectWebServerToken = "Error validating token for web server, ignoring."
	errorWritingToWebSocket      = "Error writing to websocket: "
)

const (
	responseInfo = iota
	responseError
)

type IncomingJSON struct {
	Token   string
	Command string
	ID      string
}

type OutgoingJSON struct {
	Status  int
	Message string
}

func webServer() {
	if !conf.webserverConfigured() {
		logThis(webServerNotConfigured, NORMAL)
		return
	}

	rtr := mux.NewRouter()
	if conf.webServer.allowDownloads {
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
			if token[0] != conf.webServer.token {
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
				// TODO if server is shutting down, c.Close()

				incoming := IncomingJSON{}
				if err := c.ReadJSON(&incoming); err != nil {
					if websocket.IsCloseError(err, websocket.CloseGoingAway) {
						break
					}
					logThis(errorIncomingWebSocketJSON+err.Error(), NORMAL)
					continue
				}
				if incoming.Token != conf.webServer.token {
					logThis(errorIncorrectWebServerToken, NORMAL)
					continue
				}

				// DEBUG!!
				log.Printf("recv: %s %s", incoming.Command, incoming.ID)

				switch incoming.Command {
				case handshakeCommand:
					hello := OutgoingJSON{Status: responseInfo, Message: handshakeCommand}
					if err := c.WriteJSON(hello); err != nil {
						logThis(errorWritingToWebSocket+err.Error(), NORMAL)
					}
				case downloadCommand:
					fmt.Println("GET")
					// TODO go snatch like from http cli (maybe even just redirect?)
					// TODO write back status
					success := OutgoingJSON{Status: responseInfo, Message: "Successfully snatched torrent!"}
					if err := c.WriteJSON(success); err != nil {
						logThis(errorWritingToWebSocket+err.Error(), NORMAL)
					}

				case statsCommand:
					fmt.Println("STATS")
					// TODO gather stats and send text / or svgs (ie snatched today, this week, etc...)
				default:
					fmt.Println("ERROR")
					hello := OutgoingJSON{Status: responseError, Message: errorUnknownCommand + incoming.Command}
					if err := c.WriteJSON(hello); err != nil {
						logThis(errorWritingToWebSocket+err.Error(), NORMAL)
					}
				}

			}
		}
		// interface for remotely ordering downloads
		rtr.HandleFunc("/get/{id:[0-9]+}", getTorrent).Methods("GET")
		rtr.HandleFunc("/dl.pywa", getTorrent).Methods("GET")
		rtr.HandleFunc("/ws", socket)
	}
	if conf.webServer.serveStats {
		// serving static index.html in stats dir
		if conf.webServer.statsPassword != "" {
			rtr.PathPrefix("/").Handler(httpauth.SimpleBasicAuth(conf.user, conf.webServer.statsPassword)(http.FileServer(http.Dir(statsDir))))
		} else {
			rtr.PathPrefix("/").Handler(http.FileServer(http.Dir(statsDir)))
		}
	}

	// serve
	if conf.serveHTTP() {
		go func() {
			logThis(webServerUpHTTP, NORMAL)
			serverHTTP = &http.Server{Addr: fmt.Sprintf(":%d", conf.webServer.portHTTP), Handler: rtr}
			if err := serverHTTP.ListenAndServe(); err != nil {
				if err == http.ErrServerClosed {
					logThis(webServerShutDown, NORMAL)
				} else {
					logThis(errorServing+err.Error(), NORMAL)
				}
			}
		}()
	}
	if conf.serveHTTPS() {
		// if not there yet, generate the self-signed certificate
		if !FileExists(filepath.Join(certificatesDir, certificateKey)) || !FileExists(filepath.Join(certificatesDir, certificate)) {
			if err := generateCertificates(); err != nil {
				logThis(errorGeneratingCertificate+err.Error()+provideCertificate, NORMAL)
				logThis(infoBackupScript, NORMAL)
				return
			}
			// basic instruction for first connection.
			logThis(infoAddCertificates, NORMAL)
		}

		go func() {
			logThis(webServerUpHTTPS, NORMAL)
			serverHTTPS = &http.Server{Addr: fmt.Sprintf(":%d", conf.webServer.portHTTPS), Handler: rtr}
			if err := serverHTTPS.ListenAndServeTLS(filepath.Join(certificatesDir, certificate), filepath.Join(certificatesDir, certificateKey)); err != nil {
				if err == http.ErrServerClosed {
					logThis(webServerShutDown, NORMAL)
				} else {
					logThis(errorServing+err.Error(), NORMAL)
				}
			}
		}()
	}
	logThis(webServersUp, NORMAL)
}
