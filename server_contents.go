package main

import (
	"io/ioutil"
	"path/filepath"
	"strconv"
	"time"

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
					<li class="pure-menu-item"><a class="pure-menu-link" href="#title">{{.Title}}</a></li>
			{{range .Stats}}
				<li class="pure-menu-heading">{{.Name}}</li>
				<li class="pure-menu-item"> <a class="pure-menu-link" href="#stats-{{ .Name }}">Stats</a></li>
				{{range .GraphLinks}}
				<li class="pure-menu-item"> <a class="pure-menu-link" href="{{ .URL }}">{{ .Name }}</a></li>
				{{end}}
			{{end}}
				</ul>
			</div>
		</div>

		<div id="main">
		  <div class="header">
				<h1 id="title">{{.Title}}</h1>
				<p>Last updated: {{.Time}}</p>
				<p>Raw data: {{range .CSV}}<a href="{{ .URL }}">[{{ .Name }}]</a> {{else}}{{end}}</p>
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

type ServerData struct {
	index HTMLIndex
	theme HistoryTheme
}

func (sc *ServerData) Index(e *Environment) ([]byte, error) {
	// updating time
	sc.index.Time = time.Now().Format("2006-01-02 15:04:05")
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

	err := sc.index.SetMainContentStats()
	if err != nil {
		return []byte{}, errors.Wrap(err, "Error generating stats page")
	}
	// building and returning complete page
	return sc.index.MainPage()
}

func (sc *ServerData) SaveIndex(e *Environment, file string) error {
	// building index
	data, err := sc.Index(e)
	if err != nil {
		return nil
	}
	// write to file
	return ioutil.WriteFile(file, data, 0644)
}

func (sc *ServerData) DownloadsList(e *Environment) ([]byte, error) {
	// TODO
	main, err := sc.index.MainPage()
	if err != nil {
		return []byte{}, errors.Wrap(err, "Error generating main page")
	}
	//fmt.Println(string(main))

	/*	list := "<h1>Downloads</h1><ul>"
		*	for _, d := range e.Downloads.Downloads {
					list += fmt.Sprintf(`<li><a href="downloads/%d">%s</a></li>`, d.Index, d.RawShortString())
				}
				list += "</ul>"
	*/
	return main, nil // []byte(list), nil

}

func (sc *ServerData) DownloadsInfo(e *Environment, id string) ([]byte, error) {
	// TODO

	// display individual download metadata
	downloadID, err := strconv.ParseUint(id, 10, 64)
	if err != nil {
		return []byte{}, errors.New("Error parsing download ID")
	}
	// find Download
	dl, err := e.Downloads.FindByID(downloadID)
	if err != nil {
		return []byte{}, errors.New("Error finding download ID " + id + " in db.")
	}

	response := []byte{}
	if dl.HasDescription {
		// TODO if more than 1 tracker, make things prettier
		for _, rinfo := range dl.ReleaseInfo {
			response = append(response, blackfriday.MarkdownCommon(rinfo)...)
		}
	} else {
		response = []byte(dl.RawShortString())
	}
	return response, nil

}
