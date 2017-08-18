package varroa

import (
	"html/template"

	"bufio"
	"bytes"

	"github.com/pkg/errors"
	"github.com/tdewolff/minify"
	"github.com/tdewolff/minify/css"
	"github.com/tdewolff/minify/html"
	"github.com/tdewolff/minify/js"
	"github.com/tdewolff/minify/svg"
)

// adapted from https://purecss.io/layouts/side-menu/
// color palette from https://material.io/color/#!/?view.left=0&view.right=0&primary.color=F57F17&secondary.color=37474F&primary.text.color=000000&secondary.text.color=ffffff
const (
	htlmStatsTemplate = `
		{{range .Stats}}
		<h1 id="stats-{{.Name}}" >{{.Name}}</h1>

		<h2  class="content-subhead">{{.Name}} Stats</h2>
		<table class="stats-table" summary="Last stats for {{.Name}}">
		    <thead>
		      <tr>
				<th>Date</th>
				<th>Upload</th>
				<th>Download</th>
				<th>Buffer</th>
				<th>Warning Buffer</th>
				<th>Ratio</th>
		      </tr>
		    </thead>
		    <tbody>
		{{range .TrackerStats}}
			{{range $index, $value := .}}
				{{if eq $index 0 }}
					{{if eq $value "+"}}
					<tr class="good-stats">
					{{else}}
					<tr class="bad-stats">
					{{end}}
				{{else}}
				<td>{{.}}</td>
				{{end}}
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
`
	htlmDownloadsListTemplate = `
		<h1>Downloads with full tracker metadata</h1>
		<ul>
		{{range .Downloads.Downloads}}
			{{ if .HasDescription}}
			<li>
				[{{.ShortState}}] <a href="downloads/{{.Index}}">{{.Path}}</a>
			</li>
			{{ end }}
		{{end}}
		</ul>
`
	htlmDownloadsInfoTemplate = `
		{{.DownloadInfo}}
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
	Title         string
	Time          string
	Version       string
	CSV           []HTMLLink
	Stats         []HTMLStats
	CSS           template.CSS
	Script        string
	ShowDownloads bool
	Downloads     []DownloadEntry
	DownloadInfo  template.HTML
	MainContent   template.HTML
}

func (hi *HTMLIndex) execute(t *template.Template) ([]byte, error) {
	// open file
	b := new(bytes.Buffer)
	writer := bufio.NewWriter(b)
	// write to []byte
	if err := t.Execute(writer, hi); err != nil {
		return []byte{}, errors.Wrap(err, "Error executing template for index")
	}
	// flushing is very important.
	writer.Flush()
	return b.Bytes(), nil
}

// IndexStats executes the template and save the result to a file.
func (hi *HTMLIndex) IndexStats() ([]byte, error) {
	t, err := template.New("index").Parse(htlmStatsTemplate)
	if err != nil {
		return []byte{}, errors.Wrap(err, "Error generating template for index")
	}
	return hi.execute(t)
}

func (hi *HTMLIndex) SetMainContentStats() error {
	stats, err := hi.IndexStats()
	if err != nil {
		return err
	}
	hi.MainContent = template.HTML(stats)
	return nil
}

func (hi *HTMLIndex) IndexDownloadsList() ([]byte, error) {
	if len(hi.Downloads) == 0 {
		return []byte{}, errors.New("Error generating downloads list: nothing found")
	}

	t, err := template.New("index_dl").Parse(htlmDownloadsListTemplate)
	if err != nil {
		return []byte{}, errors.Wrap(err, "Error generating template for index")
	}
	return hi.execute(t)
}

func (hi *HTMLIndex) SetMainContentDownloadsList() error {
	dlList, err := hi.IndexDownloadsList()
	if err != nil {
		return err
	}
	hi.MainContent = template.HTML(dlList)
	return nil
}

func (hi *HTMLIndex) IndexDownloadsInfo() ([]byte, error) {
	if len(hi.DownloadInfo) == 0 {
		return []byte{}, errors.New("Error generating downloads info: nothing found")
	}

	t, err := template.New("index_dlinfo").Parse(htlmDownloadsInfoTemplate)
	if err != nil {
		return []byte{}, errors.Wrap(err, "Error generating template for index")
	}
	return hi.execute(t)
}

func (hi *HTMLIndex) SetMainContentDownloadsInfo() error {
	dlInfo, err := hi.IndexDownloadsInfo()
	if err != nil {
		return err
	}
	hi.MainContent = template.HTML(dlInfo)
	return nil
}

func (hi *HTMLIndex) MainPage() ([]byte, error) {
	if len(hi.MainContent) == 0 {
		return []byte{}, errors.New("Error generating template for index: no main content")
	}

	t, err := template.New("index_main").Parse(htlmTemplate)
	if err != nil {
		return []byte{}, errors.Wrap(err, "Error generating template for index")
	}
	pageBytes, err := hi.execute(t)
	if err != nil {
		return []byte{}, err
	}

	// minify output
	m := minify.New()
	m.AddFunc("text/css", css.Minify)
	m.AddFunc("text/html", html.Minify)
	m.AddFunc("text/javascript", js.Minify)
	m.AddFunc("image/svg+xml", svg.Minify)
	min, err := m.Bytes("text/html", pageBytes)
	if err != nil {
		return pageBytes, nil
	}
	return min, nil
}
