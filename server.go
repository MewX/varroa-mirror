package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/goji/httpauth"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
)

const (
	webServerNotConfigured = "No configuration found for the web server."
	webServerShutDown      = "Web server has closed."
	webServerUpHTTP        = "Starting http web server."
	webServerUpHTTPS       = "Starting https web server."
	webServersUp           = "Web server(s) started."
	errorServing           = "Error launching web interface: "
	errorWrongToken        = "Error receiving download order from https: wrong token"
	errorNoToken           = "Error receiving download order from https: no token"
	errorNoID              = "Error retreiving torrent ID"
	errorNoStatsFilename   = "Error retreiving stats filename "

	downloadCommand  = "get"
	handshakeCommand = "hello"
	statsCommand     = "stats"

	errorUnknownCommand          = "Error: unknown websocket command: "
	errorIncomingWebSocketJSON   = "Error parsing websocket input: "
	errorIncorrectWebServerToken = "Error validating token for web server, ignoring."
	errorWritingToWebSocket      = "Error writing to websocket: "
	errorCreatingWebSocket       = "Error creating websocket: "

	autoCloseTab = "<html><head><script>t = null;function moveMe(){t = setTimeout(\"self.close()\",5000);}</script></head><body onload=\"moveMe()\">Successfully downloaded torrent: %s</body></html>"
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

// TODO: see if this could also be used by irc
func snatchFromID(id string) (*Release, error) {
	// get torrent info
	info, err := tracker.GetTorrentInfo(id)
	if err != nil {
		logThis(errorCouldNotGetTorrentInfo, NORMAL)
		return nil, err // probably the ID does not exist
	}
	release := info.Release()
	if release == nil {
		logThis("Error parsing Torrent Info", NORMAL)
		release = &Release{TorrentID: id}
	}
	release.torrentURL = conf.url + "/torrents.php?action=download&id=" + id
	release.TorrentFile = "remote-id" + id + ".torrent"

	logThis("Web server: downloading torrent "+release.ShortString(), NORMAL)
	if err := tracker.Download(release); err != nil {
		logThis(errorDownloadingTorrent+release.torrentURL+" /  "+err.Error(), NORMAL)
		return release, err
	}
	// move to relevant watch directory
	if err := CopyFile(release.TorrentFile, filepath.Join(conf.defaultDestinationFolder, release.TorrentFile)); err != nil {
		logThis(errorCouldNotMoveTorrent+err.Error(), NORMAL)
		return release, err
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
	// save metadata
	saveTrackerMetadata(info)
	return release, nil
}

func validateGet(r *http.Request) (string, error) {
	queryParameters := r.URL.Query()
	// get torrent ID
	id, ok := mux.Vars(r)["id"]
	if !ok {
		// if it's not in URL, try to get from query parameters
		queryID, ok2 := queryParameters["id"]
		if !ok2 {
			return "", errors.New(errorNoID)
		}
		id = queryID[0]
	}
	// checking token
	token, ok := queryParameters["token"]
	if !ok {
		// try to get token from "pass" parameter instead
		token, ok = queryParameters["pass"]
		if !ok {
			return "", errors.New(errorNoToken)
		}
	}
	if token[0] != conf.webServer.token {
		return "", errors.New(errorWrongToken)
	}
	return id, nil
}

func webServer() {
	if !conf.webserverConfigured() {
		logThis(webServerNotConfigured, NORMAL)
		return
	}

	rtr := mux.NewRouter()
	if conf.webServer.allowDownloads {
		getStats := func(w http.ResponseWriter, r *http.Request) {
			// checking token
			token, ok := r.URL.Query()["token"]
			if !ok {
				logThis(errorNoToken, NORMAL)
				w.WriteHeader(http.StatusNotFound)
				return
			}
			if token[0] != conf.webServer.token {
				logThis(errorWrongToken, NORMAL)
				w.WriteHeader(http.StatusNotFound)
				return
			}
			filename, ok := mux.Vars(r)["name"]
			if !ok {
				logThis(errorNoStatsFilename, NORMAL)
				w.WriteHeader(http.StatusNotFound)
				return
			}
			file, err := ioutil.ReadFile(filepath.Join(statsDir, filename))
			if err != nil {
				logThis(errorNoStatsFilename+": "+err.Error(), NORMAL)
				w.WriteHeader(http.StatusNotFound)
				return
			}
			if strings.HasSuffix(filename, svgExt) {
				w.Header().Set("Content-type", "image/svg")
			} else {
				w.Header().Set("Content-type", "image/png")
			}
			w.Write(file)
		}
		getTorrent := func(w http.ResponseWriter, r *http.Request) {
			id, err := validateGet(r)
			if err != nil {
				logThis("Error parsing request: "+err.Error(), NORMAL)
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			// snatching
			release, err := snatchFromID(id)
			if err != nil {
				logThis("Error snatching trt: "+err.Error(), NORMAL)
				return
			}
			// write response
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(fmt.Sprintf(autoCloseTab, release.ShortString())))
		}
		upgrader := websocket.Upgrader{
			// allows connection to websocket from anywhere
			CheckOrigin: func(r *http.Request) bool { return true },
		}
		socket := func(w http.ResponseWriter, r *http.Request) {
			c, err := upgrader.Upgrade(w, r, nil)
			if err != nil {
				logThis(errorCreatingWebSocket+err.Error(), NORMAL)
				return
			}
			defer c.Close()
			websocketOutput = true
			// channel to know when the connection with a specific instance is over
			endThisConnection := make(chan struct{})

			// this goroutine will send messages to the remote
			go func() {
				for {
					select {
					case messageToLog := <-sendToWebsocket:
						// TODO differentiate info / error
						if err := c.WriteJSON(OutgoingJSON{Status: responseInfo, Message: messageToLog}); err != nil {
							logThis(errorWritingToWebSocket+err.Error(), NORMAL)
						}
					case <-endThisConnection:
						return
					}
				}
			}()

			for {
				// TODO if server is shutting down, c.Close()

				incoming := IncomingJSON{}
				if err := c.ReadJSON(&incoming); err != nil {
					if websocket.IsCloseError(err, websocket.CloseGoingAway) || websocket.IsCloseError(err, websocket.CloseAbnormalClosure) {
						endThisConnection <- struct{}{}
						websocketOutput = false
						break
					}
					logThis(errorIncomingWebSocketJSON+err.Error(), NORMAL)
					continue
				}

				var answer OutgoingJSON
				if incoming.Token != conf.webServer.token {
					logThis(errorIncorrectWebServerToken, NORMAL)
					answer = OutgoingJSON{Status: responseError, Message: "Bad token!"}
				} else {
					// dealing with command
					switch incoming.Command {
					case handshakeCommand:
						// say hello right back
						answer = OutgoingJSON{Status: responseInfo, Message: handshakeCommand}
					case downloadCommand:
						// snatching
						release, err := snatchFromID(incoming.ID)
						if err != nil {
							logThis("Error snatching torrent: "+err.Error(), NORMAL)
							answer = OutgoingJSON{Status: responseError, Message: "Error snatching torrent."}
						} else {
							answer = OutgoingJSON{Status: responseInfo, Message: "Successfully snatched torrent " + release.ShortString()}
						}
					case statsCommand:
						answer = OutgoingJSON{Status: responseInfo, Message: "STATS!"}
						// TODO gather stats and send text / or svgs (ie snatched today, this week, etc...)
					default:
						answer = OutgoingJSON{Status: responseError, Message: errorUnknownCommand + incoming.Command}
					}
				}
				// writing answer
				if err := c.WriteJSON(answer); err != nil {
					logThis(errorWritingToWebSocket+err.Error(), NORMAL)
				}

				// TODO: reset after a while
			}
		}
		// interface for remotely ordering downloads
		rtr.HandleFunc("/get/{id:[0-9]+}", getTorrent).Methods("GET")
		rtr.HandleFunc("/getStats/{name:[\\w]+.svg}", getStats).Methods("GET")
		rtr.HandleFunc("/getStats/{name:[\\w]+.png}", getStats).Methods("GET")
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
