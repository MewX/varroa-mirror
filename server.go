package main

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gorilla/mux"
)

const (
	webServerNotConfigured = "No configuration found for the web server."
	errorServing           = "Error launching web interface: "
)

func webServer(tracker GazelleTracker) {
	if !conf.webserverConfigured() {
		logThis(webServerNotConfigured, NORMAL)
		return
	}
	rtr := mux.NewRouter()
	if conf.webServerAllowDownloads {
		// interface for remotely ordering downloads
		rtr.HandleFunc("/get/{id:[0-9]+}", func(w http.ResponseWriter, r *http.Request) {
			params := mux.Vars(r)
			id := params["id"]
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
			// adding to history ?
			// NOTE: or do we keep history for autosnatching only? would require filling in the Release struct from the info JSON
			// NOTE: this would allow sending release.ShortString to the notification later
			//if err := history.SnatchHistory.Add(release, "remote"); err != nil {
			//	logThis(errorAddingToHistory, NORMAL)
			//}
			// send notification
			if err := notification.Send("Snatched with web interface " + "torrent #" + id); err != nil {
				logThis(errorNotification+err.Error(), VERBOSE)
			}
			// save metadata once the download folder is created
			saveTrackerMetadata(info)
		}).Methods("GET")
	}
	if conf.webServerServeStats {
		// serving static index.html in stats dir
		rtr.PathPrefix("/").Handler(http.FileServer(http.Dir(statsDir)))
	}
	http.Handle("/", rtr)
	// serve
	if err := http.ListenAndServe(fmt.Sprintf(":%d", conf.webServerPort), nil); err != nil {
		logThis(errorServing+err.Error(), NORMAL)
	}
}
