package varroa

import (
	"html/template"
	"io/ioutil"
	"path/filepath"
	"strconv"

	"github.com/pkg/errors"
	"github.com/russross/blackfriday"
)

// adapted from https://purecss.io/layouts/side-menu/
// color palette from https://material.io/color/#!/?view.left=0&view.right=0&primary.color=F57F17&secondary.color=37474F&primary.text.color=000000&secondary.text.color=ffffff
const (
	indexJS = `
(function (window, document) {
    var layout   = document.getElementById('layout'),
	menu     = document.getElementById('menu'),
	menuLink = document.getElementById('menuLink'),
	content  = document.getElementById('main');

    function toggleClass(element, className) {
	var classes = element.className.split(/\s+/),
	    length = classes.length,
	    i = 0;

	for(; i < length; i++) {
	  if (classes[i] === className) {
	    classes.splice(i, 1);
	    break;
	  }
	}
	// The className is not found
	if (length === classes.length) {
	    classes.push(className);
	}

	element.className = classes.join(' ');
    }

    function toggleAll(e) {
	var active = 'active';

	e.preventDefault();
	toggleClass(layout, active);
	toggleClass(menu, active);
	toggleClass(menuLink, active);
    }

    menuLink.onclick = function (e) {
	toggleAll(e);
    };

    content.onclick = function(e) {
	if (menu.className.indexOf('active') !== -1) {
	    toggleAll(e);
	}
    };
}(this, this.document));
	`

	htlmTemplate = `
<!doctype html>
<html lang="en">
  <head>
    <title>varroa musica</title>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <link rel="stylesheet" href="https://unpkg.com/purecss@0.6.2/build/pure-min.css" integrity="sha384-UQiGfs9ICog+LwheBSRCt1o5cbyKIHbwjWscjemyBMT9YCUMZffs6UqUTd0hObXD" crossorigin="anonymous">
    <style>
       {{.CSS}}
    </style>
  </head>
  <body>

	<div id="layout">
		<!-- Menu toggle -->
		<a href="#menu" id="menuLink" class="menu-link">
			<!-- Hamburger icon -->
			<span></span>
		</a>

		<div id="menu">
			<div class="pure-menu">
				<ul class="pure-menu-list">
					<li class="pure-menu-item"><a class="pure-menu-link" href="/{{.UrlFolder}}#title">{{.Title}}</a></li>
				{{if .ShowDownloads }}
					<li class="pure-menu-item"><a class="pure-menu-link" href="downloads">Downloads</a></li>
				{{end}}
				{{range .Stats}}
					<li class="pure-menu-heading">{{.Name}}</li>
					<li class="pure-menu-item"> <a class="pure-menu-link" href="/{{$.UrlFolder}}#stats-{{ .Name }}">Stats</a></li>
					{{range .GraphLinks}}
					<li class="pure-menu-item"> <a class="pure-menu-link" href="/{{$.UrlFolder}}{{ .URL }}">{{ .Name }}</a></li>
					{{end}}
				{{end}}
				</ul>
			</div>
		</div>

		<div id="main">
		  <div class="header">
				<h1 id="title">{{.Title}}</h1>
				<p>Graphs last updated: {{.Time}}</p>
				<p>Raw data: {{range .CSV}}<a href="/{{$.UrlFolder}}{{ .URL }}">[{{ .Name }}]</a> {{else}}{{end}}</p>
				<p>{{.Version}}</p>
		  </div>
		  <div class="content">
			{{.MainContent}}
		  </div>
		</div>

	</div>

	<script>{{.Script}}</script>
  </body>
</html>
`
)

type ServerPage struct {
	index HTMLIndex
	theme HistoryTheme
}

func (sc *ServerPage) update(e *Environment, downloads *Downloads) {
	// updating time
	e.mutex.RLock()
	sc.index.Time = e.graphsLastUpdated
	e.mutex.RUnlock()
	// rebuilding
	sc.index.CSV = []HTMLLink{}
	sc.index.Stats = []HTMLStats{}
	if e.config.webserverMetadata && downloads != nil {
		// fetch all dl entries
		if err := downloads.DB.All(&sc.index.Downloads); err != nil {
			logThis.Error(err, NORMAL)
		} else {
			sc.index.ShowDownloads = true
		}
	}
	// gathering data
	for label, h := range e.History {
		sc.index.CSV = append(sc.index.CSV, HTMLLink{Name: label + ".csv", URL: filepath.Base(h.getPath(statsFile + csvExt))})

		statsNames := []struct {
			Name  string
			Label string
		}{
			{Name: "Buffer", Label: label + "_" + bufferStatsFile},
			{Name: "Upload", Label: label + "_" + uploadStatsFile},
			{Name: "Download", Label: label + "_" + downloadStatsFile},
			{Name: "Ratio", Label: label + "_" + ratioStatsFile},
			{Name: "Buffer/day", Label: label + "_" + bufferPerDayStatsFile},
			{Name: "Upload/day", Label: label + "_" + uploadPerDayStatsFile},
			{Name: "Download/day", Label: label + "_" + downloadPerDayStatsFile},
			{Name: "Ratio/day", Label: label + "_" + ratioPerDayStatsFile},
			{Name: "Snatches/day", Label: label + "_" + numberSnatchedPerDayFile},
			{Name: "Size Snatched/day", Label: label + "_" + sizeSnatchedPerDayFile},
		}
		// add graphs + links
		graphLinks := []HTMLLink{}
		graphs := []HTMLLink{}
		for _, s := range statsNames {
			graphLinks = append(graphLinks, HTMLLink{Name: s.Name, URL: "#" + s.Label})
			graphs = append(graphs, HTMLLink{Title: label + ": " + s.Name, Name: s.Label, URL: s.Label + svgExt})
		}
		// add previous stats (progress)
		var lastStats []*TrackerStats
		var lastStatsStrings [][]string
		if len(h.TrackerStats) < 25 {
			lastStats = h.TrackerStats
		} else {
			lastStats = h.TrackerStats[len(h.TrackerStats)-25 : len(h.TrackerStats)]
		}
		for i, s := range lastStats {
			if i == 0 {
				continue
			}
			lastStatsStrings = append(lastStatsStrings, s.ProgressParts(lastStats[i-1]))
		}
		// reversing
		for left, right := 0, len(lastStatsStrings)-1; left < right; left, right = left+1, right-1 {
			lastStatsStrings[left], lastStatsStrings[right] = lastStatsStrings[right], lastStatsStrings[left]
		}
		// TODO timestamps: first column for h.TrackerRecords.

		stats := HTMLStats{Name: label, TrackerStats: lastStatsStrings, Graphs: graphs, GraphLinks: graphLinks}
		sc.index.Stats = append(sc.index.Stats, stats)
	}
}

func (sc *ServerPage) Index(e *Environment, downloads *Downloads) ([]byte, error) {
	// updating
	sc.update(e, downloads)
	if err := sc.index.SetMainContentStats(); err != nil {
		return []byte{}, errors.Wrap(err, "Error generating stats page")
	}
	// building and returning complete page
	return sc.index.MainPage()
}

// SaveIndex is only used for Gitlab pages, so it never shows Downloads and will need to know the repository name (ie Pages subfolder).
func (sc *ServerPage) SaveIndex(e *Environment, file string) error {
	// building index
	if e.config.gitlabPagesConfigured {
		e.serverData.index.UrlFolder = e.config.GitlabPages.Folder + "/"
	}
	data, err := sc.Index(e, nil)
	if err != nil {
		return err
	}
	if e.config.gitlabPagesConfigured {
		e.serverData.index.UrlFolder = ""
	}
	// write to file
	return ioutil.WriteFile(file, data, 0666)
}

func (sc *ServerPage) DownloadsList(e *Environment, downloads *Downloads) ([]byte, error) {
	// updating
	sc.update(e, downloads)
	// getting downloads
	if err := sc.index.SetMainContentDownloadsList(); err != nil {
		return []byte{}, errors.Wrap(err, "Error generating downloads list page")
	}
	// building and returning complete page
	return sc.index.MainPage()
}

func (sc *ServerPage) DownloadsInfo(e *Environment, downloads *Downloads, id string) ([]byte, error) {
	// updating
	sc.update(e, nil)

	// display individual download metadata
	downloadID, err := strconv.Atoi(id)
	if err != nil {
		return []byte{}, errors.New("Error parsing download ID")
	}
	// find Download
	dl, err := downloads.FindByID(downloadID)
	if err != nil {
		return []byte{}, errors.New("Error finding download ID " + id + " in db.")
	}
	// get description
	sc.index.DownloadInfo = ""
	if dl.HasTrackerMetadata {
		// TODO if more than 1 tracker, make things prettier
		for _, t := range dl.Tracker {
			sc.index.DownloadInfo += template.HTML(blackfriday.MarkdownCommon(dl.getDescription(e.config.General.DownloadDir, t)))
		}
	} else {
		sc.index.DownloadInfo = template.HTML(dl.RawShortString())
	}

	// getting info
	if err := sc.index.SetMainContentDownloadsInfo(); err != nil {
		return []byte{}, errors.Wrap(err, "Error generating downloads info page")
	}
	// building and returning complete page
	return sc.index.MainPage()
}
