package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/goji/httpauth"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
	"github.com/subosito/norma"
	"github.com/russross/blackfriday"
)

const (
	downloadCommand  = "get"
	handshakeCommand = "hello"
	statsCommand     = "stats"
	autoCloseTab     = "<html><head><script>t = null;function moveMe(){t = setTimeout(\"self.close()\",5000);}</script></head><body onload=\"moveMe()\">Successfully downloaded torrent: %s</body></html>"
)

const (
	responseInfo = iota
	responseError
)

// IncomingJSON from the websocket created by the GM script, also used with unix socket.
type IncomingJSON struct {
	Token   string
	Command string
	Args    []string
	Site    string
}

// OutgoingJSON to the websocket created by the GM script.
type OutgoingJSON struct {
	Status  int
	Message string
}

// TODO: see if this could also be used by irc
func snatchFromID(e *Environment, tracker *GazelleTracker, id string) (*Release, error) {
	// get torrent info
	info, err := tracker.GetTorrentInfo(id)
	if err != nil {
		logThis.Info(errorCouldNotGetTorrentInfo, NORMAL)
		return nil, err // probably the ID does not exist
	}
	release := info.Release()
	if release == nil {
		logThis.Info("Error parsing Torrent Info", NORMAL)
		release = &Release{TorrentID: id}
	}
	release.torrentURL = tracker.URL + "/torrents.php?action=download&id=" + id
	release.TorrentFile = norma.Sanitize(tracker.Name) + "_id" + id + torrentExt

	logThis.Info("Downloading torrent "+release.ShortString(), NORMAL)
	if err := tracker.DownloadTorrent(release, e.config.General.WatchDir); err != nil {
		logThis.Error(errors.Wrap(err, errorDownloadingTorrent+release.torrentURL), NORMAL)
		return release, err
	}
	// add to history
	if err := e.History[tracker.Name].AddSnatch(release, "remote"); err != nil {
		logThis.Info(errorAddingToHistory, NORMAL)
	}
	// send notification
	e.Notify("Snatched with web interface: "+release.ShortString(), tracker.Name, "info")
	// save metadata
	if e.config.General.AutomaticMetadataRetrieval {
		if e.inDaemon {
			go release.Metadata.SaveFromTracker(tracker, info, e.config.General.DownloadDir)
		} else {
			release.Metadata.SaveFromTracker(tracker, info, e.config.General.DownloadDir)
		}
	}
	return release, nil
}

func validateGet(r *http.Request, config *Config) (string, string, error) {
	queryParameters := r.URL.Query()
	// get torrent ID
	id, ok := mux.Vars(r)["id"]
	if !ok {
		// if it's not in URL, try to get from query parameters
		queryID, ok2 := queryParameters["id"]
		if !ok2 {
			return "", "", errors.New(errorNoID)
		}
		id = queryID[0]
	}
	// get site
	trackerLabel, ok := mux.Vars(r)["site"]
	if !ok {
		// if it's not in URL, try to get from query parameters
		queryTrackerLabel, ok2 := queryParameters["site"]
		if !ok2 {
			return "", "", errors.New(errorNoID)
		}
		trackerLabel = queryTrackerLabel[0]
	}
	// checking token
	token, ok := queryParameters["token"]
	if !ok {
		// try to get token from "pass" parameter instead
		token, ok = queryParameters["pass"]
		if !ok {
			return "", "", errors.New(errorNoToken)
		}
	}
	if token[0] != config.WebServer.Token {
		return "", "", errors.New(errorWrongToken)
	}

	return trackerLabel, id, nil
}

func webServer(e *Environment, httpServer *http.Server, httpsServer *http.Server) {
	if !e.config.webserverConfigured {
		logThis.Info(webServerNotConfigured, NORMAL)
		return
	}
	if e.config.WebServer.ServeMetadata {
		if err := e.Downloads.Load(filepath.Join(statsDir, downloadsDBFile+msgpackExt)); err != nil {
			logThis.Error(errors.Wrap(err, "Error loading downloads database"), NORMAL)
			return
		}
		// scan on startup in goroutine
		go e.Downloads.Scan()
	}

	rtr := mux.NewRouter()
	var mutex = &sync.Mutex{}
	if e.config.WebServer.AllowDownloads {
		getStats := func(w http.ResponseWriter, r *http.Request) {
			// checking token
			token, ok := r.URL.Query()["token"]
			if !ok {
				logThis.Info(errorNoToken, NORMAL)
				w.WriteHeader(http.StatusNotFound)
				return
			}
			if token[0] != e.config.WebServer.Token {
				logThis.Info(errorWrongToken, NORMAL)
				w.WriteHeader(http.StatusNotFound)
				return
			}
			// get site
			trackerLabel, ok := mux.Vars(r)["site"]
			if !ok {
				// if it's not in URL, try to get from query parameters
				queryTrackerLabel, ok2 := r.URL.Query()["site"]
				if !ok2 {
					logThis.Info(errorNoID, NORMAL)
					w.WriteHeader(http.StatusNotFound)
					return
				}
				trackerLabel = queryTrackerLabel[0]
			}
			// get filename
			filename, ok := mux.Vars(r)["name"]
			if !ok {
				logThis.Info(errorNoStatsFilename, NORMAL)
				w.WriteHeader(http.StatusNotFound)
				return
			}
			file, err := ioutil.ReadFile(filepath.Join(statsDir, trackerLabel+"_"+filename))
			if err != nil {
				logThis.Error(errors.Wrap(err, errorNoStatsFilename), NORMAL)
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
			trackerLabel, id, err := validateGet(r, e.config)
			if err != nil {
				logThis.Error(errors.Wrap(err, "Error parsing request"), NORMAL)
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			// snatching
			tracker, err := e.Tracker(trackerLabel)
			if err != nil {
				logThis.Error(errors.Wrap(err, "Error identifying in configuration tracker "+trackerLabel), NORMAL)
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			release, err := snatchFromID(e, tracker, id)
			if err != nil {
				logThis.Error(errors.Wrap(err, errorSnatchingTorrent), NORMAL)
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			// write response
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(fmt.Sprintf(autoCloseTab, release.ShortString())))
		}
		getMetadata := func(w http.ResponseWriter, r *http.Request) {
			// if not configured, return error
			if !e.config.WebServer.ServeMetadata {
				logThis.Error(errors.New("Error, not configured to serve metadata"), NORMAL)
				w.WriteHeader(http.StatusUnauthorized)
				return
			}

			response := []byte{}
			id, ok := mux.Vars(r)["id"]
			if !ok {
				list := "<h1>Downloads</h1><ul>"
				for _, d := range e.Downloads.Downloads {
					list += fmt.Sprintf(`<li><a href="downloads/%d">%s</a></li>`, d.Index, d.Path)
				}
				list += "</ul>"
				response = []byte(list)
			} else {
				// display individual download metadata
				downloadID, err := strconv.ParseUint(id, 10, 64)
				if err != nil {
					logThis.Error(errors.New("Error parsing download ID"), NORMAL)
					w.WriteHeader(http.StatusUnauthorized)
					return
				}
				// find Download
				dl, err := e.Downloads.FindByID(downloadID)
				if err != nil {
					logThis.Error(errors.New("Error finding download ID "+id+" in db."), NORMAL)
					w.WriteHeader(http.StatusUnauthorized)
					return
				}

				if dl.HasDescription {
					// TODO if more than 1 tracker, make things prettier
					for _, rinfo := range dl.ReleaseInfo {
						response = append(response, blackfriday.MarkdownCommon(rinfo)...)
					}
				} else {
					response = []byte(dl.String())
				}
			}
			// write response
			w.WriteHeader(http.StatusOK)
			w.Write(response)
		}
		upgrader := websocket.Upgrader{
			// allows connection to websocket from anywhere
			CheckOrigin: func(r *http.Request) bool { return true },
		}
		socket := func(w http.ResponseWriter, r *http.Request) {
			c, err := upgrader.Upgrade(w, r, nil)
			if err != nil {
				logThis.Error(errors.Wrap(err, errorCreatingWebSocket), NORMAL)
				return
			}
			defer c.Close()
			e.websocketOutput = true
			// channel to know when the connection with a specific instance is over
			endThisConnection := make(chan struct{})

			// this goroutine will send messages to the remote
			go func() {
				for {
					select {
					case messageToLog := <-e.sendToWebsocket:
						if e.websocketOutput {
							mutex.Lock()
							// TODO differentiate info / error
							if err := c.WriteJSON(OutgoingJSON{Status: responseInfo, Message: messageToLog}); err != nil {
								if !websocket.IsCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
									logThis.Error(errors.Wrap(err, errorIncomingWebSocketJSON), VERBOSEST)
								}
							}
							mutex.Unlock()
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
					if !websocket.IsCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
						logThis.Error(errors.Wrap(err, errorIncomingWebSocketJSON), VERBOSEST)
					}
					endThisConnection <- struct{}{}
					e.websocketOutput = false
					break
				}

				var answer OutgoingJSON
				if incoming.Token != e.config.WebServer.Token {
					logThis.Info(errorIncorrectWebServerToken, NORMAL)
					answer = OutgoingJSON{Status: responseError, Message: "Bad token!"}
				} else {
					// dealing with command
					switch incoming.Command {
					case handshakeCommand:
						// say hello right back
						answer = OutgoingJSON{Status: responseInfo, Message: handshakeCommand}
					case downloadCommand:
						tracker, err := e.Tracker(incoming.Site)
						if err != nil {
							logThis.Error(errors.Wrap(err, "Error identifying in configuration tracker "+incoming.Site), NORMAL)
							answer = OutgoingJSON{Status: responseError, Message: "Error snatching torrent."}
						} else {
							// snatching
							for _, id := range incoming.Args {
								release, err := snatchFromID(e, tracker, id)
								if err != nil {
									logThis.Info("Error snatching torrent: "+err.Error(), NORMAL)
									answer = OutgoingJSON{Status: responseError, Message: "Error snatching torrent."}
								} else {
									answer = OutgoingJSON{Status: responseInfo, Message: "Successfully snatched torrent " + release.ShortString()}
								}
								// TODO send responses for all IDs (only 1 from GM Script for now anyway)
							}
						}
					case statsCommand:
						answer = OutgoingJSON{Status: responseInfo, Message: "STATS!"}
						// TODO gather stats and send text / or svgs (ie snatched today, this week, etc...)
					default:
						answer = OutgoingJSON{Status: responseError, Message: errorUnknownCommand + incoming.Command}
					}
				}
				// writing answer
				mutex.Lock()
				if err := c.WriteJSON(answer); err != nil {
					if !websocket.IsCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
						logThis.Error(errors.Wrap(err, errorIncomingWebSocketJSON), VERBOSEST)
					}
				}
				mutex.Unlock()
			}
		}
		// interface for remotely ordering downloads
		rtr.HandleFunc("/get/{id:[0-9]+}", getTorrent).Methods("GET")
		rtr.HandleFunc("/downloads", getMetadata).Methods("GET")
		rtr.HandleFunc("/downloads/{id:[0-9]+}", getMetadata).Methods("GET")
		rtr.HandleFunc("/getStats/{name:[\\w]+.svg}", getStats).Methods("GET")
		rtr.HandleFunc("/getStats/{name:[\\w]+.png}", getStats).Methods("GET")
		rtr.HandleFunc("/dl.pywa", getTorrent).Methods("GET")
		rtr.HandleFunc("/ws", socket)
	}
	if e.config.WebServer.ServeStats {
		// serving static index.html in stats dir
		if e.config.WebServer.Password != "" {
			rtr.PathPrefix("/").Handler(httpauth.SimpleBasicAuth(e.config.WebServer.User, e.config.WebServer.Password)(http.FileServer(http.Dir(statsDir))))
		} else {
			rtr.PathPrefix("/").Handler(http.FileServer(http.Dir(statsDir)))
		}
	}
	// serve
	if e.config.webserverHTTP {
		go func() {
			logThis.Info(webServerUpHTTP, NORMAL)
			httpServer = &http.Server{Addr: fmt.Sprintf(":%d", e.config.WebServer.PortHTTP), Handler: rtr}
			if err := httpServer.ListenAndServe(); err != nil {
				if err == http.ErrServerClosed {
					logThis.Info(webServerShutDown, NORMAL)
				} else {
					logThis.Error(errors.Wrap(err, errorServing), NORMAL)
				}
			}
		}()
	}
	if e.config.webserverHTTPS {
		// if not there yet, generate the self-signed certificate
		if !FileExists(filepath.Join(certificatesDir, certificateKey)) || !FileExists(filepath.Join(certificatesDir, certificate)) {
			if err := generateCertificates(e); err != nil {
				logThis.Error(errors.Wrap(err, errorGeneratingCertificate+provideCertificate), NORMAL)
				logThis.Info(infoBackupScript, NORMAL)
				return
			}
			// basic instruction for first connection.
			logThis.Info(infoAddCertificates, NORMAL)
		}

		go func() {
			logThis.Info(webServerUpHTTPS, NORMAL)
			httpsServer = &http.Server{Addr: fmt.Sprintf(":%d", e.config.WebServer.PortHTTPS), Handler: rtr}
			if err := httpsServer.ListenAndServeTLS(filepath.Join(certificatesDir, certificate), filepath.Join(certificatesDir, certificateKey)); err != nil {
				if err == http.ErrServerClosed {
					logThis.Info(webServerShutDown, NORMAL)
				} else {
					logThis.Error(errors.Wrap(err, errorServing), NORMAL)
				}
			}
		}()
	}
	logThis.Info(webServersUp, NORMAL)
}
