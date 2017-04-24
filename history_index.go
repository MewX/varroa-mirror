package main

import (
	"html/template"
	"os"

	"github.com/pkg/errors"
)

const htlmIndexTemplate = `
<html>
  <head>
    <title>varroa musica</title>
    <meta content="">
    <style></style>
  </head>
  <body>
    <h1 style="text-align:center;">{{.Title}}</h1>
    <p id="title" style="text-align:center;">Last updated: {{.Time}}{{range .CSV}} | <a href="{{ .URL }}">{{ .Name }}</a>{{else}}{{end}}</p>


{{range .Stats}}
	<p style="text-align:center;">Latest {{.Name}} stats: {{.Stats}}</p>
{{end}}
{{range .Stats}}
	<p style="text-align:center;">{{.Name}}{{range .GraphLinks}} | <a href="{{ .URL }}">{{ .Name }}</a>{{end}}</p>
{{end}}

{{range .Stats}}
	{{range .Graphs}}
		<p id="{{.Name}}" style="text-align:center;"><img src="{{.URL}}" alt="missing stats" style="align:center"></p>
		<p style="text-align:center;"><i>{{.Title}}</i> <a href="#title">&uarr;</a></p>
	{{end}}
{{end}}


  </body>
</html>`

type HTMLLink struct {
	Name  string
	URL   string
	Title string
}

type HTMLStats struct {
	Name       string
	Stats      string
	GraphLinks []HTMLLink
	Graphs     []HTMLLink
}

type HTMLIndex struct {
	Title string
	Time  string
	CSV   []HTMLLink
	Stats []HTMLStats
}

func (hi *HTMLIndex) ToHTML(file string) error {
	t, err := template.New("index").Parse(htlmIndexTemplate)
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
