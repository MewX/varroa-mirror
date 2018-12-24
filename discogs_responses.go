package varroa

import (
	"strconv"
)

type DiscogsResults struct {
	Pagination struct {
		Items   int      `json:"items"`
		Page    int      `json:"page"`
		Pages   int      `json:"pages"`
		PerPage int      `json:"per_page"`
		Urls    struct{} `json:"urls"`
	} `json:"pagination"`
	Results []struct {
		Barcode   []string `json:"barcode"`
		Catno     string   `json:"catno"`
		Community struct {
			Have int `json:"have"`
			Want int `json:"want"`
		} `json:"community"`
		Country     string      `json:"country"`
		CoverImage  string      `json:"cover_image"`
		Format      []string    `json:"format"`
		Genre       []string    `json:"genre"`
		ID          int         `json:"id"`
		Label       []string    `json:"label"`
		MasterID    int         `json:"master_id"`
		MasterURL   interface{} `json:"master_url"`
		ResourceURL string      `json:"resource_url"`
		Style       []string    `json:"style"`
		Thumb       string      `json:"thumb"`
		Title       string      `json:"title"`
		Type        string      `json:"type"`
		URI         string      `json:"uri"`
		UserData    struct {
			InCollection bool `json:"in_collection"`
			InWantlist   bool `json:"in_wantlist"`
		} `json:"user_data"`
		Year string `json:"year"`
	} `json:"results"`
}

type DiscogsRelease struct {
	Artists []struct {
		Anv         string `json:"anv"`
		ID          int    `json:"id"`
		Join        string `json:"join"`
		Name        string `json:"name"`
		ResourceURL string `json:"resource_url"`
		Role        string `json:"role"`
		Tracks      string `json:"tracks"`
	} `json:"artists"`
	ArtistsSort string `json:"artists_sort"`
	Community   struct {
		Contributors []struct {
			ResourceURL string `json:"resource_url"`
			Username    string `json:"username"`
		} `json:"contributors"`
		DataQuality string `json:"data_quality"`
		Have        int    `json:"have"`
		Rating      struct {
			Average float64 `json:"average"`
			Count   int     `json:"count"`
		} `json:"rating"`
		Status    string `json:"status"`
		Submitter struct {
			ResourceURL string `json:"resource_url"`
			Username    string `json:"username"`
		} `json:"submitter"`
		Want int `json:"want"`
	} `json:"-"`
	Companies []struct {
		Catno          string `json:"catno"`
		EntityType     string `json:"entity_type"`
		EntityTypeName string `json:"entity_type_name"`
		ID             int    `json:"id"`
		Name           string `json:"name"`
		ResourceURL    string `json:"resource_url"`
	} `json:"companies"`
	Country         string `json:"country"`
	DataQuality     string `json:"data_quality"`
	DateAdded       string `json:"date_added"`
	DateChanged     string `json:"date_changed"`
	EstimatedWeight int    `json:"estimated_weight"`
	Extraartists    []struct {
		Anv         string `json:"anv"`
		ID          int    `json:"id"`
		Join        string `json:"join"`
		Name        string `json:"name"`
		ResourceURL string `json:"resource_url"`
		Role        string `json:"role"`
		Tracks      string `json:"tracks"`
	} `json:"extraartists"`
	FormatQuantity int `json:"format_quantity"`
	Formats        []struct {
		Descriptions []string `json:"descriptions"`
		Name         string   `json:"name"`
		Qty          string   `json:"qty"`
	} `json:"formats"`
	Genres      []string `json:"genres"`
	ID          int      `json:"id"`
	Identifiers []struct {
		Type  string `json:"type"`
		Value string `json:"value"`
	} `json:"identifiers"`
	Images []struct {
		Height      int    `json:"height"`
		ResourceURL string `json:"resource_url"`
		Type        string `json:"type"`
		URI         string `json:"uri"`
		URI150      string `json:"uri150"`
		Width       int    `json:"width"`
	} `json:"images"`
	Labels []struct {
		Catno          string `json:"catno"`
		EntityType     string `json:"entity_type"`
		EntityTypeName string `json:"entity_type_name"`
		ID             int    `json:"id"`
		Name           string `json:"name"`
		ResourceURL    string `json:"resource_url"`
	} `json:"labels"`
	LowestPrice       float64       `json:"-"`
	MasterID          int           `json:"master_id"`
	MasterURL         string        `json:"master_url"`
	Notes             string        `json:"notes"`
	NumForSale        int           `json:"-"`
	Released          string        `json:"released"`
	ReleasedFormatted string        `json:"released_formatted"`
	ResourceURL       string        `json:"resource_url"`
	Series            []interface{} `json:"series"`
	Status            string        `json:"status"`
	Styles            []string      `json:"styles"`
	Thumb             string        `json:"thumb"`
	Title             string        `json:"title"`
	Tracklist         []struct {
		Artists []struct {
			Anv         string `json:"anv"`
			ID          int    `json:"id"`
			Join        string `json:"join"`
			Name        string `json:"name"`
			ResourceURL string `json:"resource_url"`
			Role        string `json:"role"`
			Tracks      string `json:"tracks"`
		} `json:"artists,omitempty"`
		Duration string `json:"duration"`
		Position string `json:"position"`
		Title    string `json:"title"`
		Type     string `json:"type_"`
	} `json:"tracklist"`
	URI    string `json:"uri"`
	Videos []struct {
		Description string `json:"description"`
		Duration    int    `json:"duration"`
		Embed       bool   `json:"embed"`
		Title       string `json:"title"`
		URI         string `json:"uri"`
	} `json:"-"`
	Year int `json:"year"`
}

func (dr DiscogsRelease) TrackTags() []TrackTags {
	var trackTags []TrackTags
	if len(dr.Tracklist) == 0 {
		logThis.Info("discogs data not available", VERBOSE)
	} else {
		for _, t := range dr.Tracklist {
			var tags TrackTags
			tags.Number = t.Position
			tags.TotalTracks = strconv.Itoa(len(dr.Tracklist))
			// TODO tags.DiscNumber = ????
			if len(t.Artists) != 0 {
				tags.Artist = t.Artists[0].Name // TODO what if more than one?
			} else {
				tags.Artist = dr.Artists[0].Name // TODO what if more than one?
			}
			tags.AlbumArtist = dr.Artists[0].Name // TODO what if more than one?
			tags.Title = t.Title
			tags.Description = ""
			tags.Year = strconv.Itoa(dr.Year)
			tags.Genre = dr.Genres[0] // TODO what if more than one?
			tags.Performer = ""       // TODO see how to fill this
			tags.Composer = ""        // TODO same
			tags.Album = dr.Title
			tags.Label = dr.Labels[0].Name

			trackTags = append(trackTags, tags)
		}
	}
	return trackTags
}
