package main

import (
	"fmt"
	"html/template"
	"os"

	"github.com/pkg/errors"
)

// adapted from https://purecss.io/layouts/side-menu/
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

	indexCSS = `
body {
    /*color: #777;*/
    background-color: #37474f;
}

.legend {
	text-align: center;
}
.legend a {
	text-decoration: none;
}

.pure-img-responsive {
    max-width: 100%;
    height: auto;
}

/*
Add transition to containers so they can push in and out.
*/
#layout,
#menu,
.menu-link {
    -webkit-transition: all 0.2s ease-out;
    -moz-transition: all 0.2s ease-out;
    -ms-transition: all 0.2s ease-out;
    -o-transition: all 0.2s ease-out;
    transition: all 0.2s ease-out;
}

/*
This is the parent <div> that contains the menu and the content area.
*/
#layout {
    position: relative;
    left: 0;
    padding-left: 0;
}
    #layout.active #menu {
	left: 150px;
	width: 150px;
    }

    #layout.active .menu-link {
	left: 150px;
    }
/*
The content <div> is where all your content goes.
*/
.content {
    margin: 0 auto;
    padding: 0 2em;
    max-width: 800px;
    margin-bottom: 50px;
    line-height: 1.6em;
}

.header {
     margin: 0;
    /* color: #333;*/
     text-align: center;
     padding: 2.5em 2em 0;
     border-bottom: 1px solid #eee;
 }
    .header h1 {
	margin: 0.2em 0;
	font-size: 3em;
	font-weight: 300;
    }
     .header h2 {
	font-weight: 300;
	color: #ccc;
	padding: 0;
	margin-top: 0;
    }

.content-subhead {
    margin: 50px 0 20px 0;
    font-weight: 300;
    color: #ddd; /*#888;*/
}



/*
The #menu <div> is the parent <div> that contains the .pure-menu that
appears on the left side of the page.
*/

#menu {
    margin-left: -150px; /* "#menu" width */
    width: 150px;
    position: fixed;
    top: 0;
    left: 0;
    bottom: 0;
    z-index: 1000; /* so the menu or its navicon stays above all content */
    background: #f57f17; /*#191818;*/
    overflow-y: auto;
    -webkit-overflow-scrolling: touch;
}
    /*
    All anchors inside the menu should be styled like this.
    */
    #menu a {
	color: #000000; /* #999;*/
	border: none;
	padding: 0.6em 0 0.6em 0.6em;
    }

    /*
    Remove all background/borders, since we are applying them to #menu.
    */
     #menu .pure-menu,
     #menu .pure-menu ul {
	border: none;
	background: transparent;
    }

    /*
    Add that light border to separate items into groups.
    */
    #menu .pure-menu ul,
    #menu .pure-menu .menu-item-divided {
	border-top: 1px solid #333;
    }
	/*
	Change color of the anchor links on hover/focus.
	*/
	#menu .pure-menu li a:hover,
	#menu .pure-menu li a:focus {
	    background: #bc5100; /*#333;*/
	}

    /*
    This styles the selected menu item <li>.
    */
    #menu .pure-menu-selected,
    #menu .pure-menu-heading {
	background: #ffb04c; /*#1f8dd6;*/
    }
	/*
	This styles a link within a selected menu item <li>.
	*/
	#menu .pure-menu-selected a {
	    color: #fff;
	}

    /*
    This styles the menu heading.
    */
    #menu .pure-menu-heading {
	font-size: 110%;
	color: #fff;
	margin: 0;
    }

/* -- Dynamic Button For Responsive Menu -------------------------------------*/

/*
The button to open/close the Menu is custom-made and not part of Pure. Here's
how it works:
*/

/*
.menu-link represents the responsive menu toggle that shows/hides on
small screens.
*/
.menu-link {
    position: fixed;
    display: block; /* show this only on small screens */
    top: 0;
    left: 0; /* "#menu width" */
    background: #000;
    background: rgba(0,0,0,0.7);
    font-size: 10px; /* change this value to increase/decrease button size */
    z-index: 10;
    width: 2em;
    height: auto;
    padding: 2.1em 1.6em;
}

    .menu-link:hover,
    .menu-link:focus {
	background: #000;
    }

    .menu-link span {
	position: relative;
	display: block;
    }

    .menu-link span,
    .menu-link span:before,
    .menu-link span:after {
	background-color: #fff;
	width: 100%;
	height: 0.2em;
    }

	.menu-link span:before,
	.menu-link span:after {
	    position: absolute;
	    margin-top: -0.6em;
	    content: " ";
	}

	.menu-link span:after {
	    margin-top: 0.6em;
	}


/* -- Responsive Styles (Media Queries) ------------------------------------- */

/*
Hides the menu at 48em, but modify this based on your app's needs.
*/
@media (min-width: 48em) {

    .header,
    .content {
	padding-left: 2em;
	padding-right: 2em;
	color: #fff;
    }

    #layout {
	padding-left: 150px; /* left col width "#menu" */
	left: 0;
    }
    #menu {
	left: 150px;
    }

    .menu-link {
	position: fixed;
	left: 150px;
	display: none;
    }

    #layout.active .menu-link {
	left: 150px;
    }
}

@media (max-width: 48em) {
    /* Only apply this when the window is small. Otherwise, the following
    case results in extra padding on the left:
	* Make the window small.
	* Tap the menu to trigger the active state.
	* Make the window large again.
    */
    #layout.active {
	position: relative;
	left: 150px;
    }
}

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
			<li class="pure-menu-item"> <a class="pure-menu-link" href="#stats-{{ .Name }}">stats</a></li>
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
           	<h2>Complete stats.</h2>
        </div>
        <div class="content">
		<h2 class="content-subhead">Last Updated</h2>
		<p>Last updated: {{.Time}}{{range .CSV}} | <a href="{{ .URL }}">{{ .Name }}</a>{{else}}{{end}}</p>

		<h2 class="content-subhead">Stats</h2>
		{{range .Stats}}
		<p id="stats-{{.Name}}">Latest {{.Name}} stats: {{.Stats}}</p>
		{{end}}

		{{range .Stats}}
		<h2 class="content-subhead">{{.Name}} Graphs</h2>
		<h3 class="content-subhead">Preview</h3>
		<div class="pure-g">
			{{range .Graphs}}
			<div class="pure-u-1-6">
				<a href="#{{ .Name }}"><img class="pure-img-responsive" src="{{.URL}}" alt="<missing stats, not enough data yet?>"></a>
			</div>
			{{end}}
		</div>
		<h3 class="content-subhead">Graphs</h3>
		{{range .Graphs}}
		<div class="pure-g">
			<div class="pure-u-1-1" id="{{.Name}}">
				<img class="pure-img-responsive" src="{{.URL}}" alt="<missing stats, not enough data yet?>" style="align:center">
			</div>
		</div>
		<p class="legend"><i>{{.Title}}</i> <a href="#title">&uarr;</a></p>
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
	t, err := template.New("index").Parse(fmt.Sprintf(htlmIndexTemplate, indexCSS, indexJS))
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
