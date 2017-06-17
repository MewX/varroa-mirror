package main

import (
	"fmt"
	"html/template"
	"os"

	"github.com/pkg/errors"
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
	htlmIndexTemplate = `
<!doctype html>
<html lang="en">
  <head>
    <title>varroa musica</title>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <link rel="stylesheet" href="https://unpkg.com/purecss@0.6.2/build/pure-min.css" integrity="sha384-UQiGfs9ICog+LwheBSRCt1o5cbyKIHbwjWscjemyBMT9YCUMZffs6UqUTd0hObXD" crossorigin="anonymous">
    <style>%s</style>
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
		{{range .Stats}}
		<h1 id="stats-{{.Name}}" >{{.Name}}</h1>

		<h2  class="content-subhead">{{.Name}} Stats</h2>
		<table class="hourly_statistics_table">
		    <thead>
		      <tr>
			<th>Upload</th>
			<th>Download</th>
			<th>Buffer</th>
			<th>Warning Buffer</th>
			<th>Ratio</th>
		      </tr>
		    </thead>
		    <tbody>
		{{range .TrackerStats}}
		<tr>
			{{range .}}
			<td>{{.}}</td>
			{{end}}
		</tr>
		{{end}}
		</tbody>
		</table>

		<h2 class="content-subhead">{{.Name}} Graphs</h2>
		<h3 class="content-subhead">Preview</h3>
		<div class="pure-g">
			{{range .Graphs}}
			<div class="pure-u-1-6">
				<a href="#{{ .Name }}"><img class="pure-img-responsive" src="{{.URL}}" alt="<missing stats, not enough data yet?>" title="{{.Name}}"></a>
			</div>
			{{end}}
		</div>



		<h3 class="content-subhead">Graphs</h3>
		{{range .Graphs}}

		<div class="pure-g">
			<div class="pure-u-1" id="{{.Name}}">
				<a href="#openModal-{{.Name}}"><img class="pure-img-responsive" src="{{.URL}}" alt="<missing stats, not enough data yet?>" style="align:center"></a>
			</div>
		</div>
		<p class="legend"><i>{{.Title}}</i> <a href="#title">&uarr;</a></p>

		<!-- Modal window to make the graph fullscreen -->
		<div id="openModal-{{.Name}}" class="modalDialog">
			<a href="#close" title="Close" class="close">X</a>
			<div class="pure-g">
				<div class="pure-u-1">
					<img class="pure-img-responsive" src="{{.URL}}" alt="<missing stats, not enough data yet?>">
				</div>
			</div>
		</div>

		{{end}}
		{{end}}
        </div>
    </div>
</div>

<script>%s</script>

</body>
</html>
`
)

// HTMLLink represents a link.
type HTMLLink struct {
	Name  string
	URL   string
	Title string
}

// HTMLStats has all the information for a tracker: stats and graphs.
type HTMLStats struct {
	Name         string
	TrackerStats [][]string
	GraphLinks   []HTMLLink
	Graphs       []HTMLLink
}

// HTMLIndex provides data for the htmlIndexTemplate.
type HTMLIndex struct {
	Title   string
	Time    string
	Version string
	CSV     []HTMLLink
	Stats   []HTMLStats
	Theme   HistoryTheme
}

// ToHTML executes the template and save the result to a file.
func (hi *HTMLIndex) ToHTML(file string) error {
	t, err := template.New("index").Parse(fmt.Sprintf(htlmIndexTemplate, hi.Theme.CSS(), indexJS))
	if err != nil {
		return errors.Wrap(err, "Error generating template for index")
	}
	// open file
	f, err := os.OpenFile(file, os.O_RDWR|os.O_TRUNC|os.O_CREATE, 0644)
	if err != nil {
		return errors.Wrap(err, "Error opening index file tor writing")
	}
	// write to file
	return t.Execute(f, hi)
}
