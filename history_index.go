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
    <link rel="stylesheet" href="https://unpkg.com/purecss@0.6.2/build/base-min.css">
    <link rel="stylesheet" href="https://unpkg.com/purecss@0.6.2/build/grids-responsive-min.css">

    <style>
body {
    margin: 0;
    padding: 0;
    font-family:'Abel', arial, serif;
    text-transform: uppercase;
    color: white;
    background: #546e7a; /* url(../img/fond-menu.png) no-repeat top center;*/
}

#stats-menu {
    /*height: 150px;*/
 /*   width: 250px;*/
  /*  padding-top: 50px;*/
    /*margin: 5px 5px 5px 5px;*/
   position: fixed;
}
#menu-title {
    list-style: none;
    padding: 0px;
    color: blue;
  /*  line-height: 15px;*/
    font-size: 15px;
}

#stats-menu ul {
    padding: 0;
    margin: 0;
    margin-left: 10px;
    float: left;
}

#stats-menu ul li {
    list-style: none;
    padding: 0px;
    color: #fff176;
  /*  line-height: 20px;*/
    font-size: 15px;
}
#stats-menu ul li a {
    text-decoration: none;
    font-size: 15px;
    padding: 4px;
    display: block;
    color: white;
    background: transparent;
    width: 150px;
    -moz-transition: all .5s;
    -webkit-transition: all .5s;
    -o-transition: all .5s;
    transition: all .5s;
  /*  line-height: 15px;*/
}
#stats-menu ul li a:hover {
    background: #819ca9;
    padding-left: 10px;
    width: 130px;
    -moz-transition: all .5s;
    -webkit-transition: all .5s;
    -o-transition: all .5s;
    transition: all .5s;
}
.graph {
	text-align:center;
}
    </style>
  </head>
  <body>

<div class="pure-g">

<div class="pure-u-1-5">

	 <div id="stats-menu">

		<ul>
			<li id="menu-title"><a href="#title">{{.Title}}</a></li>
	{{range .Stats}}

			<li>{{.Name}}</li>
			<li> <a href="#stats-{{ .Name }}">Stats</a></li>
			{{range .GraphLinks}}
			<li> <a href="{{ .URL }}">{{ .Name }}</a></li>
			{{end}}

	{{end}}
		</ul>
	</div>
</div>

<div class="pure-u-4-5">
	<div id="content">
		 <h1 id="title" class="graph">{{.Title}}</h1>
		 <p class="graph">Last updated: {{.Time}}{{range .CSV}} | <a href="{{ .URL }}">{{ .Name }}</a>{{else}}{{end}}</p>
		{{range .Stats}}
			<p id="stats-{{.Name}}" class="graph">Latest {{.Name}} stats: {{.Stats}}</p>
		{{end}}

		{{range .Stats}}
			{{range .Graphs}}
				<p id="{{.Name}}" class="graph"><img src="{{.URL}}" alt="missing stats" style="align:center"></p>
				<p class="graph"><i>{{.Title}}</i> <a href="#title">&uarr;</a></p>
			{{end}}
		{{end}}
	</div>

</div>
</div>

  </body>
</html>`

// HTMLLink represents a link.
type HTMLLink struct {
	Name  string
	URL   string
	Title string
}

// HTMLStats has all the information for a tracker: stats and graphs.
type HTMLStats struct {
	Name       string
	Stats      string
	GraphLinks []HTMLLink
	Graphs     []HTMLLink
}

// HTMLIndex provides data for the htmlIndexTemplate.
type HTMLIndex struct {
	Title string
	Time  string
	CSV   []HTMLLink
	Stats []HTMLStats
}

// ToHTML executes the template and save the result to a file.
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
