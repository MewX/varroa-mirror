package main

import (
	"bytes"
	"html/template"
)

const (
	darkOrange = "dark_orange"
	darkGreen  = "dark_green"
	lightBlue  = "light_blue"

	CSSTemplate = `
	body {
	    	background-color: {{.IndexBackgroundColor}};
	}
	p {
		color: {{.IndexFontColor}};
	}
	.legend {
		text-align: center;
	}
	.legend a {
		text-decoration: none;
		color: #ccc;
	}

	.pure-img-responsive {
	    	max-width: 100%;
	   	height: auto;
	   	text-align: center;
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
		max-width: 1000px;
		margin-bottom: 50px;
		color: {{.IndexFontColor}};
	}
	.content ul {
   	 	list-style: none;
    	padding: 0;
    	margin: 0;
	}
	.content li {
		padding-left: 16px;
	}
	.content li:before {
		content: "↪";
		padding-right: 8px;
		color: {{.IndexMenuColor}};
	}
	.content a {
		text-decoration: none;
		color: #ccc;
	}
	.content a:hover {
		text-decoration: underline;
		text-decoration-style: dashed;
	}
	.content p img {
		display: block;
		max-width: 80%;
		margin-left: auto;
        margin-right: auto;
	}
	.header {
		margin: 0;
		text-align: center;
		padding: 2.5em 2em 0;
		border-bottom: 1px solid #eee;
		color: {{.IndexFontColor}};
	 }
	.header h1 {
		margin: 0.2em 0;
		font-size: 3em;
		font-weight: 300;
		color: {{.IndexFontColor}};
	}

	.header h2 {
		font-weight: 300;
		color: #ccc;
		padding: 0;
		margin-top: 0;
	}
	.header a {
		text-decoration: none;
		color: #ccc;
	}

	.content-subhead {
	    	margin: 50px 0 20px 0;
	    	font-weight: 300;
	    	color: #ddd;
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
		background: {{.IndexMenuColor}};
		overflow-y: auto;
		-webkit-overflow-scrolling: touch;
	}
	/*
	All anchors inside the menu should be styled like this.
	*/
	#menu a {
		color: #000000;
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
	    	background: {{.IndexMenuLinkColor}};
	}

	/*
	This styles the selected menu item <li>.
	*/
	#menu .pure-menu-selected,
	#menu .pure-menu-heading {
		background: {{.IndexMenuBackgroundColor}};
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
		color: {{.IndexMenuHeaderColor}};
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
		/* Only apply this when the window is small. Otherwise, the following case results in extra padding on the left:
		* Make the window small.
		* Tap the menu to trigger the active state.
		* Make the window large again.
		*/
		#layout.active {
			position: relative;
			left: 150px;
		}
	}
	/* lightbox */
	.modalDialog {
		position: fixed;
		font-family: Arial, Helvetica, sans-serif;
		top: 0;
		right: 0;
		bottom: 0;
		left: 0;
		padding: 10px;
		background: {{.IndexBackgroundColor}};
		z-index: 99998;
		opacity:0;
		-webkit-transition: opacity 400ms ease-in;
		-moz-transition: opacity 400ms ease-in;
		transition: opacity 400ms ease-in;
		pointer-events: none;
	}
	.modalDialog:target {
		opacity:1;
		pointer-events: auto;
	}
	.modalDialog > div {
		padding: 10px;
		border-radius: 10px;
		background: {{.IndexBackgroundColor}};
		position: relative;
		top: 50%;
		transform: perspective(1px) translateY(-50%);
		margin-top: 10px;
		margin-bottom: 10px;
		margin-left: auto;
		margin-right: auto;
		display: flex;
		flex-flow: row wrap;
		justify-content: space-around;
		text-align: center;
	}
	.close {
		z-index: 99999;
		background: #606061;
		color: #FFFFFF;
		line-height: 25px;
		position: fixed;
		right: 5px;
		text-align: center;
		top: 5px;
		width: 24px;
		text-decoration: none;
		font-weight: bold;
		-webkit-border-radius: 12px;
		-moz-border-radius: 12px;
		border-radius: 12px;
		-moz-box-shadow: 1px 1px 3px #000;
		-webkit-box-shadow: 1px 1px 3px #000;
		box-shadow: 1px 1px 3px #000;
	}
	.close:hover { background: #00d9ff; }

	/* table */
	.stats-table
	{
		font-size: 0.8em;
		font-weight: normal;
		text-align: left;
		border-collapse: collapse;
		border: 1px solid {{.GraphColor}};
	}
	.stats-table th
	{
		padding: 10px;
		color: {{.IndexFontColor}};
		border-bottom: 1px dashed {{.GraphColor}};
	}
	.stats-table td
	{
		padding: 10px;
		color: {{.IndexFontColor}};
	}
	.stats-table tbody tr:hover td
	{
		color: {{.IndexFontColor}};
		background: {{.GraphColor}};
	}
	.good-stats {

	}
	.bad-stats {
		font-weight: bold;
		border: 2px dashed red;
	}


`
)

type HistoryTheme struct {
	GraphTransparentBackground bool
	GraphColor                 string
	GraphFillerOpacity         uint8
	GraphAxisColor             string
	IndexBackgroundColor       string
	IndexFontColor             string
	IndexMenuColor             string
	IndexMenuLinkColor         string
	IndexMenuBackgroundColor   string
	IndexMenuHeaderColor       string
}

var (
	darkOrangeTheme = HistoryTheme{
		GraphTransparentBackground: true,
		GraphColor:                 "#f57f17",
		GraphFillerOpacity:         80,
		GraphAxisColor:             "#ffffff",
		IndexBackgroundColor:       "#37474f",
		IndexFontColor:             "white",
		IndexMenuColor:             "#f57f17",
		IndexMenuLinkColor:         "#bc5100",
		IndexMenuBackgroundColor:   "#ffb04c",
		IndexMenuHeaderColor:       "#ffffff",
	}
	darkGreenTheme = HistoryTheme{
		GraphTransparentBackground: true,
		GraphColor:                 "#00aa00",
		GraphFillerOpacity:         80,
		GraphAxisColor:             "#fff",
		IndexBackgroundColor:       "#222",
		IndexFontColor:             "white",
		IndexMenuColor:             "#666",
		IndexMenuLinkColor:         "white",
		IndexMenuBackgroundColor:   "#222",
		IndexMenuHeaderColor:       "#666",
	}
	lightBlueTheme = HistoryTheme{
		GraphTransparentBackground: true,
		GraphColor:                 "#0074D9",
		GraphFillerOpacity:         25,
		GraphAxisColor:             "#000",
		IndexBackgroundColor:       "#fff",
		IndexFontColor:             "#0096FB",
		IndexMenuColor:             "#0096FB",
		IndexMenuLinkColor:         "#fff",
		IndexMenuBackgroundColor:   "#fff",
		IndexMenuHeaderColor:       "#0096FB",
	}

	knownThemeNames = []string{darkGreen, darkOrange, lightBlue}

	knownThemes = map[string]HistoryTheme{
		darkOrange: darkOrangeTheme,
		darkGreen:  darkGreenTheme,
		lightBlue:  lightBlueTheme,
	}
)

func (ht HistoryTheme) CSS() template.CSS {
	var doc bytes.Buffer
	tCSS, err := template.New("css").Parse(CSSTemplate)
	if err != nil {
		return ""
	}
	if err := tCSS.Execute(&doc, ht); err != nil {
		return ""
	}
	return template.CSS(doc.String())
}
