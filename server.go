package varroa

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"strings"
	"sync"

	"github.com/goji/httpauth"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
	"github.com/subosito/norma"
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
	FLToken bool
	Site    string
}

// OutgoingJSON to the websocket created by the GM script.
type OutgoingJSON struct {
	Status  int
	Message string
}

// TODO: see if this could also be used by irc
func manualSnatchFromID(e *Environment, tracker *GazelleTracker, id string, useFLToken bool) (*Release, error) {
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
	if useFLToken {
		release.torrentURL += "&usetoken=1"
	}
	release.TorrentFile = norma.Sanitize(tracker.Name) + "_id" + id + torrentExt

	logThis.Info("Downloading torrent "+release.ShortString(), NORMAL)
	if err := tracker.DownloadTorrent(release.torrentURL, release.TorrentFile, e.config.General.WatchDir); err != nil {
		logThis.Error(errors.Wrap(err, errorDownloadingTorrent+release.torrentURL), NORMAL)
		return release, err
	}
	// add to history
	if err := e.History[tracker.Name].AddSnatch(release, "remote"); err != nil {
		logThis.Info(errorAddingToHistory, NORMAL)
	}
	// save metadata
	if e.config.General.AutomaticMetadataRetrieval {
		if e.InDaemon {
			go release.Metadata.SaveFromTracker(tracker, info, e.config.General.DownloadDir)
		} else {
			release.Metadata.SaveFromTracker(tracker, info, e.config.General.DownloadDir)
		}
	}
	return release, nil
}

func validateGet(r *http.Request, config *Config) (string, string, bool, error) {
	queryParameters := r.URL.Query()
	// get torrent ID
	id, ok := mux.Vars(r)["id"]
	if !ok {
		// if it's not in URL, try to get from query parameters
		queryID, ok2 := queryParameters["id"]
		if !ok2 {
			return "", "", false, errors.New(errorNoID)
		}
		id = queryID[0]
	}
	// get site
	trackerLabel, ok := mux.Vars(r)["site"]
	if !ok {
		// if it's not in URL, try to get from query parameters
		queryTrackerLabel, ok2 := queryParameters["site"]
		if !ok2 {
			return "", "", false, errors.New(errorNoID)
		}
		trackerLabel = queryTrackerLabel[0]
	}
	// checking token
	token, ok := queryParameters["token"]
	if !ok {
		// try to get token from "pass" parameter instead
		token, ok = queryParameters["pass"]
		if !ok {
			return "", "", false, errors.New(errorNoToken)
		}
	}
	if token[0] != config.WebServer.Token {
		return "", "", false, errors.New(errorWrongToken)
	}

	// checking FL token use
	useFLToken := false
	useIt, ok := queryParameters["fltoken"]
	if ok && useIt[0] == "true" {
		useFLToken = true
		logThis.Info("Snatching using FL Token if possible.", VERBOSE)
	}
	return trackerLabel, id, useFLToken, nil
}

func webServer(e *Environment, httpServer *http.Server, httpsServer *http.Server) {
	if !e.config.webserverConfigured {
		logThis.Info(webServerNotConfigured, NORMAL)
		return
	}
	downloads := Downloads{Root: e.config.General.DownloadDir}
	if e.config.WebServer.ServeMetadata {
		if err := downloads.Open(filepath.Join(StatsDir, DownloadsDBFile+msgpackExt)); err != nil {
			logThis.Error(errors.Wrap(err, "Error loading downloads database"), NORMAL)
			return
		}
		// scan on startup in goroutine
		go downloads.Scan()
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
			file, err := ioutil.ReadFile(filepath.Join(StatsDir, trackerLabel+"_"+filename))
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
			trackerLabel, id, useFLToken, err := validateGet(r, e.config)
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
			release, err := manualSnatchFromID(e, tracker, id, useFLToken)
			if err != nil {
				logThis.Error(errors.Wrap(err, ErrorSnatchingTorrent), NORMAL)
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
				list, err := e.serverData.DownloadsList(e, downloads)
				if err != nil {
					logThis.Error(errors.Wrap(err, "Error loading downloads list"), NORMAL)
					w.WriteHeader(http.StatusUnauthorized)
					return
				}
				response = list
			} else {

				info, err := e.serverData.DownloadsInfo(e, downloads, id)
				if err != nil {
					logThis.Error(errors.Wrap(err, "Error loading downloads info"), NORMAL)
					w.WriteHeader(http.StatusUnauthorized)
					return
				}
				response = info
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
								release, err := manualSnatchFromID(e, tracker, id, incoming.FLToken)
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
			rtr.PathPrefix("/").Handler(httpauth.SimpleBasicAuth(e.config.WebServer.User, e.config.WebServer.Password)(http.FileServer(http.Dir(StatsDir))))
		} else {
			rtr.PathPrefix("/").Handler(http.FileServer(http.Dir(StatsDir)))
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
