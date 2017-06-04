package main

type HistoryTheme struct {
	GraphTransparentBackground   bool
	GraphColor                   string
	GraphFillerOpacity           uint8
	GraphAxisColor               string
	IndexBackgroundColor         string
	IndexMenuColor               string
	IndexMenuLinkColor           string
	IndexMenuLinkBackgroundColor string
	IndexMenuHeaderColor         string
}

const (
	darkOrange = "dark_orange"
	darkGreen  = "dark_green"
	lightBlue  = "light_blue"
)

var (
	darkOrangeTheme = HistoryTheme{
		GraphTransparentBackground:   true,
		GraphColor:                   "#f57f17",
		GraphFillerOpacity:           80,
		GraphAxisColor:               "#ffffff",
		IndexBackgroundColor:         "#37474f",
		IndexMenuColor:               "#f57f17",
		IndexMenuLinkColor:           "#000000",
		IndexMenuLinkBackgroundColor: "#bc5100",
		IndexMenuHeaderColor:         "#ffb04c",
	}
	darkGreenTheme = HistoryTheme{
		GraphTransparentBackground:   true,
		GraphColor:                   "#00ff00",
		GraphFillerOpacity:           80,
		GraphAxisColor:               "#fff",
		IndexBackgroundColor:         "#222",
		IndexMenuColor:               "#292929",
		IndexMenuLinkColor:           "white",
		IndexMenuLinkBackgroundColor: "#222",
		IndexMenuHeaderColor:         "#333",
	}
	lightBlueTheme = HistoryTheme{
		GraphTransparentBackground:   true,
		GraphColor:                   "#0074D9",
		GraphFillerOpacity:           25,
		GraphAxisColor:               "#000",
		IndexBackgroundColor:         "#222",
		IndexMenuColor:               "#292929",
		IndexMenuLinkColor:           "white",
		IndexMenuLinkBackgroundColor: "#222",
		IndexMenuHeaderColor:         "#333",
	}

	knownThemeNames = []string{darkGreen, darkOrange, lightBlue}

	knownThemes = map[string]HistoryTheme{
		darkOrange: darkOrangeTheme,
		darkGreen:  darkGreenTheme,
		lightBlue:  lightBlueTheme,
	}
)
