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
	"github.com/sevlyar/go-daemon"
	"gitlab.com/catastrophic/assistance/fs"
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

const (
	notificationArea = iota
	statsArea
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
	Target  int
	Message string
}

// TODO: see if this could also be used by irc
func manualSnatchFromID(e *Environment, tracker *GazelleTracker, id string, useFLToken bool) (*Release, error) {
	stats, err := NewStatsDB(filepath.Join(StatsDir, DefaultHistoryDB))
	if err != nil {
		return nil, errors.Wrap(err, "could not access the stats database")
	}

	// get torrent info
	info, err := tracker.GetTorrentMetadata(id)
	if err != nil {
		logThis.Info(errorCouldNotGetTorrentInfo, NORMAL)
		return nil, err // probably the ID does not exist
	}
	release := info.Release()
	if release == nil {
		logThis.Info("Error parsing Torrent Info", NORMAL)
		release = &Release{Tracker: tracker.Name, TorrentID: id}
	}
	logThis.Info("Downloading torrent "+release.ShortString(), NORMAL)
	if err := tracker.DownloadTorrentFromID(id, e.config.General.WatchDir, useFLToken); err != nil {
		logThis.Error(errors.Wrap(err, errorDownloadingTorrent+id), NORMAL)
		return release, err
	}
	// add to history
	release.Filter = manualSnatchFilterName
	if err := stats.AddSnatch(*release); err != nil {
		logThis.Info(errorAddingToHistory, NORMAL)
	}
	// save metadata
	if e.config.General.AutomaticMetadataRetrieval {
		if daemon.WasReborn() {
			go info.SaveFromTracker(filepath.Join(e.config.General.DownloadDir, info.FolderName), tracker)
		} else {
			info.SaveFromTracker(filepath.Join(e.config.General.DownloadDir, info.FolderName), tracker)
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

func webServer(e *Environment) {
	if !e.config.webserverConfigured {
		logThis.Info(webServerNotConfigured, NORMAL)
		return
	}
	var additionalSources []string
	if e.config.LibraryConfigured {
		additionalSources = e.config.Library.AdditionalSources
	}
	downloads, err := NewDownloadsDB(DefaultDownloadsDB, e.config.General.DownloadDir, additionalSources)
	if err != nil {
		logThis.Error(errors.Wrap(err, "Error loading downloads database"), VERBOSE)
	}
	if e.config.WebServer.ServeMetadata {
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

			var response []byte
			id, ok := mux.Vars(r)["id"]
			if !ok {
				list, err := e.serverData.DownloadsList(downloads)
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
							if err := c.WriteJSON(OutgoingJSON{Status: responseInfo, Message: messageToLog, Target: notificationArea}); err != nil {
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
					answer = OutgoingJSON{Status: responseError, Target: notificationArea, Message: "Bad token!"}
				} else {
					// dealing with command
					switch incoming.Command {
					case handshakeCommand:
						// say hello right back
						answer = OutgoingJSON{Status: responseInfo, Target: notificationArea, Message: handshakeCommand}
					case downloadCommand:
						tracker, err := e.Tracker(incoming.Site)
						if err != nil {
							logThis.Error(errors.Wrap(err, "Error identifying in configuration tracker "+incoming.Site), NORMAL)
							answer = OutgoingJSON{Status: responseError, Target: notificationArea, Message: "Error snatching torrent."}
						} else {
							// snatching
							for _, id := range incoming.Args {
								release, err := manualSnatchFromID(e, tracker, id, incoming.FLToken)
								if err != nil {
									logThis.Info("Error snatching torrent: "+err.Error(), NORMAL)
									answer = OutgoingJSON{Status: responseError, Target: notificationArea, Message: "Error snatching torrent."}
								} else {
									answer = OutgoingJSON{Status: responseInfo, Target: notificationArea, Message: "Successfully snatched torrent " + release.ShortString()}
								}
								// TODO send responses for all IDs (only 1 from GM Script for now anyway)
							}
						}
					case statsCommand:
						// TODO gather stats and send text (ie snatched today, this week, etc...)
						answer = OutgoingJSON{Status: responseInfo, Target: statsArea, Message: statusString(e)}
					default:
						answer = OutgoingJSON{Status: responseError, Target: notificationArea, Message: errorUnknownCommand + incoming.Command}
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
		getLocalStats := func(w http.ResponseWriter, r *http.Request) {
			// get filename
			filename, ok := mux.Vars(r)["name"]
			if !ok {
				logThis.Info(errorNoStatsFilename, NORMAL)
				w.WriteHeader(http.StatusNotFound)
				return
			}
			http.ServeFile(w, r, filepath.Join(StatsDir, filename))
		}
		getIndex := func(w http.ResponseWriter, r *http.Request) {
			response, err := e.serverData.Index(downloads)
			if err != nil {
				logThis.Error(errors.Wrap(err, "Error loading downloads list"), NORMAL)
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			// write response
			w.WriteHeader(http.StatusOK)
			w.Write(response)
		}
		if e.config.WebServer.Password != "" {
			rtr.Handle("/", httpauth.SimpleBasicAuth(e.config.WebServer.User, e.config.WebServer.Password)(http.HandlerFunc(getIndex)))
			rtr.Handle("/{name:[\\w]+.svg}", httpauth.SimpleBasicAuth(e.config.WebServer.User, e.config.WebServer.Password)(http.HandlerFunc(getLocalStats)))
			rtr.Handle("/{name:[\\w]+.png}", httpauth.SimpleBasicAuth(e.config.WebServer.User, e.config.WebServer.Password)(http.HandlerFunc(getLocalStats)))
		} else {
			rtr.HandleFunc("/", getIndex)
			rtr.HandleFunc("/{name:[\\w]+.svg}", getLocalStats)
			rtr.HandleFunc("/{name:[\\w]+.png}", getLocalStats)
		}
	}
	// serve
	if e.config.webserverHTTP {
		go func() {
			logThis.Info(webServerUpHTTP, NORMAL)
			httpServer := &http.Server{Addr: fmt.Sprintf(":%d", e.config.WebServer.PortHTTP), Handler: rtr}
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
		if !fs.FileExists(filepath.Join(certificatesDir, certificateKey)) || !fs.FileExists(filepath.Join(certificatesDir, certificate)) {
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
			httpsServer := &http.Server{Addr: fmt.Sprintf(":%d", e.config.WebServer.PortHTTPS), Handler: rtr}
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
