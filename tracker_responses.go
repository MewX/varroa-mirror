package varroa

import "strings"

const (
	formatFLAC = "FLAC"
	formatMP3  = "MP3"
	formatAAC  = "AAC"
	formatAC3  = "AC3"
	formatDTS  = "DTS"

	sourceCD         = "CD"
	sourceWEB        = "WEB"
	sourceDVD        = "DVD"
	sourceVinyl      = "Vinyl"
	sourceSoundboard = "Soundboard"
	sourceDAT        = "DAT"
	sourceCassette   = "Cassette"
	sourceBluRay     = "Blu-Ray"
	sourceSACD       = "SACD"

	quality192           = "192"
	quality256           = "256"
	quality320           = "320"
	qualityAPS           = "APS (VBR)"
	qualityAPX           = "APX (VBR)"
	qualityV2            = "V2 (VBR)"
	qualityV1            = "V1 (VBR)"
	qualityV0            = "V0 (VBR)"
	qualityLossless      = "Lossless"
	quality24bitLossless = "24bit Lossless"

	releaseAlbum       = "Album"
	releaseSoundtrack  = "Soundtrack"
	releaseEP          = "EP"
	releaseAnthology   = "Anthology"
	releaseCompilation = "Compilation"
	releaseSingle      = "Single"
	releaseLive        = "Live album"
	releaseRemix       = "Remix"
	releaseBootleg     = "Bootleg"
	releaseInterview   = "Interview"
	releaseMixtape     = "Mixtape"
	releaseDemo        = "Demo"
	releaseConcert     = "Concert Recording"
	releaseDJMix       = "DJ Mix"
	releaseUnknown     = "Unknown"
)

var (
	releaseTypes = map[int]string{
		1:  releaseAlbum,
		3:  releaseSoundtrack,
		5:  releaseEP,
		6:  releaseAnthology,
		7:  releaseCompilation,
		9:  releaseSingle,
		11: releaseLive,
		13: releaseRemix,
		14: releaseBootleg,
		15: releaseInterview,
		16: releaseMixtape,
		17: releaseDemo,
		18: releaseConcert,
		19: releaseDJMix,
		21: releaseUnknown,
	}
	knownReleaseTypes = []string{releaseAlbum, releaseSoundtrack, releaseEP, releaseAnthology, releaseCompilation, releaseSingle, releaseLive, releaseRemix, releaseBootleg, releaseInterview, releaseMixtape, releaseDemo, releaseConcert, releaseDJMix, releaseUnknown}
	knownFormats      = []string{formatFLAC, formatMP3, formatAAC, formatAC3, formatDTS}
	knownSources      = []string{sourceCD, sourceWEB, sourceVinyl, sourceBluRay, sourceCassette, sourceDVD, sourceDAT, sourceSoundboard, sourceSACD}
	knownQualities    = []string{quality192, quality256, quality320, qualityAPS, qualityAPX, qualityV2, qualityV1, qualityV0, qualityLossless, quality24bitLossless}
)

func getGazelleReleaseType(value int) string {
	label, ok := releaseTypes[value]
	if !ok {
		return "Unknown value"
	}
	return label
}

type GazelleGenericResponse struct {
	Response interface{} `json:"response"`
	Status   string      `json:"status"`
	Error    string      `json:"error"`
}

type GazelleIndex struct {
	Response struct {
		Authkey       string `json:"authkey"`
		ID            int    `json:"id"`
		Notifications struct {
			Messages         int  `json:"messages"`
			NewAnnouncement  bool `json:"newAnnouncement"`
			NewBlog          bool `json:"newBlog"`
			NewSubscriptions bool `json:"newSubscriptions"`
			Notifications    int  `json:"notifications"`
		} `json:"notifications"`
		Passkey   string `json:"passkey"`
		Username  string `json:"username"`
		Userstats struct {
			Class         string  `json:"class"`
			Downloaded    int     `json:"downloaded"`
			Ratio         float64 `json:"ratio"`
			Requiredratio float64 `json:"requiredratio"`
			Uploaded      int     `json:"uploaded"`
		} `json:"userstats"`
	} `json:"response"`
	Status string `json:"status"`
}

type GazelleUserStats struct {
	Response struct {
		Avatar    string `json:"avatar"`
		Community struct {
			CollagesContrib int `json:"collagesContrib"`
			CollagesStarted int `json:"collagesStarted"`
			Groups          int `json:"groups"`
			Invited         int `json:"invited"`
			Leeching        int `json:"leeching"`
			PerfectFlacs    int `json:"perfectFlacs"`
			Posts           int `json:"posts"`
			RequestsFilled  int `json:"requestsFilled"`
			RequestsVoted   int `json:"requestsVoted"`
			Seeding         int `json:"seeding"`
			Snatched        int `json:"snatched"`
			TorrentComments int `json:"torrentComments"`
			Uploaded        int `json:"uploaded"`
		} `json:"community"`
		IsFriend bool `json:"isFriend"`
		Personal struct {
			Class        string `json:"class"`
			Donor        bool   `json:"donor"`
			Enabled      bool   `json:"enabled"`
			Paranoia     int    `json:"paranoia"`
			ParanoiaText string `json:"paranoiaText"`
			Passkey      string `json:"passkey"`
			Warned       bool   `json:"warned"`
		} `json:"personal"`
		ProfileText string `json:"profileText"`
		Ranks       struct {
			Artists    int `json:"artists"`
			Bounty     int `json:"bounty"`
			Downloaded int `json:"downloaded"`
			Overall    int `json:"overall"`
			Posts      int `json:"posts"`
			Requests   int `json:"requests"`
			Uploaded   int `json:"uploaded"`
			Uploads    int `json:"uploads"`
		} `json:"ranks"`
		Stats struct {
			Downloaded    int     `json:"downloaded"`
			JoinedDate    string  `json:"joinedDate"`
			LastAccess    string  `json:"lastAccess"`
			Ratio         string  `json:"ratio"`
			RequiredRatio float64 `json:"requiredRatio"`
			Uploaded      int     `json:"uploaded"`
		} `json:"stats"`
		Username string `json:"username"`
	} `json:"response"`
	Status string `json:"status"`
}

type GazelleTorrent struct {
	Response struct {
		Group struct {
			CatalogueNumber string `json:"catalogueNumber"`
			CategoryID      int    `json:"categoryId"`
			CategoryName    string `json:"categoryName"`
			ID              int    `json:"id"`
			IsBookmarked    bool   `json:"isBookmarked"`
			MusicInfo       struct {
				Artists []struct {
					ID   int    `json:"id"`
					Name string `json:"name"`
				} `json:"artists"`
				Composers []struct {
					ID   int    `json:"id"`
					Name string `json:"name"`
				} `json:"composers"`
				Conductor []struct {
					ID   int    `json:"id"`
					Name string `json:"name"`
				} `json:"conductor"`
				Dj []struct {
					ID   int    `json:"id"`
					Name string `json:"name"`
				} `json:"dj"`
				Producer []struct {
					ID   int    `json:"id"`
					Name string `json:"name"`
				} `json:"producer"`
				RemixedBy []struct {
					ID   int    `json:"id"`
					Name string `json:"name"`
				} `json:"remixedBy"`
				With []struct {
					ID   int    `json:"id"`
					Name string `json:"name"`
				} `json:"with"`
			} `json:"musicInfo"`
			Name        string   `json:"name"`
			RecordLabel string   `json:"recordLabel"`
			ReleaseType int      `json:"releaseType"`
			Tags        []string `json:"tags"`
			Time        string   `json:"time"`
			VanityHouse bool     `json:"vanityHouse"`
			WikiBody    string   `json:"wikiBody"`
			WikiImage   string   `json:"wikiImage"`
			Year        int      `json:"year"`
		} `json:"group"`
		Torrent struct {
			Description             string `json:"description"`
			Dynamicrange            string `json:"dynamicrange"`
			Encoding                string `json:"encoding"`
			FileCount               int    `json:"fileCount"`
			FileList                string `json:"fileList"`
			FilePath                string `json:"filePath"`
			Format                  string `json:"format"`
			FreeTorrent             bool   `json:"freeTorrent"`
			Grade                   string `json:"grade"`
			HasCue                  bool   `json:"hasCue"`
			HasLog                  bool   `json:"hasLog"`
			ID                      int    `json:"id"`
			InfoHash                string `json:"infoHash"`
			Leechers                int    `json:"leechers"`
			Lineage                 string `json:"lineage"`
			LogScore                int    `json:"logScore"`
			Matrixorrunout          string `json:"matrixorrunout"`
			Media                   string `json:"media"`
			RemasterCatalogueNumber string `json:"remasterCatalogueNumber"`
			RemasterRecordLabel     string `json:"remasterRecordLabel"`
			RemasterTitle           string `json:"remasterTitle"`
			RemasterYear            int    `json:"remasterYear"`
			Remastered              bool   `json:"remastered"`
			Reported                bool   `json:"reported"`
			Samplerate              string `json:"samplerate"`
			Scene                   bool   `json:"scene"`
			Seeders                 int    `json:"seeders"`
			Size                    int    `json:"size"`
			Snatched                int    `json:"snatched"`
			Time                    string `json:"time"`
			UserID                  int    `json:"userId"`
			Username                string `json:"username"`
		} `json:"torrent"`
	} `json:"response"`
	Status string `json:"status"`
}

func (gt *GazelleTorrent) Source() string {
	source := gt.Response.Torrent.Media
	if source == sourceCD && gt.Response.Torrent.Encoding == qualityLossless {
		if gt.Response.Torrent.HasLog && gt.Response.Torrent.HasCue && (gt.Response.Torrent.LogScore == 100 || gt.Response.Torrent.Grade == "Silver") {
			source += "+"
		}
		if gt.Response.Torrent.Grade == "Gold" {
			source += "+"
		}
	}
	return source
}

func (gt *GazelleTorrent) ShortEncoding() string {
	var format string
	switch gt.Response.Torrent.Encoding {
	case qualityLossless:
		format = "FLAC"
	case quality24bitLossless:
		format = "FLAC24"
	case qualityV0:
		format = "V0"
	case qualityV2:
		format = "V2"
	case quality320:
		format = "320"
	default:
		format = "UnF"
	}
	return format
}

func (gt *GazelleTorrent) ShortEdition() string {
	editionReplacer := strings.NewReplacer(
		"Reissue", "RE",
		"Repress", "RP",
		"Remaster", "RM",
		"Remastered", "RM",
		"Limited Edition", "LTD",
		"Deluxe", "DLX",
		"Deluxe Edition", "DLX",
		"Special Editon", "SE",
		"Bonus Tracks", "Bonus",
		"Bonus Tracks Edition", "Bonus",
		"Promo", "PR",
		"Test Pressing", "TP",
		"Self Released", "SR",
		"Box Set", "Box set",
		"Compact Disc Recordable", "CDr",
		"Japan Edition", "Japan",
		"Japan Release", "Japan",
	)
	return editionReplacer.Replace(gt.Response.Torrent.RemasterTitle)
}

type GazelleTorrentGroup struct {
	Response struct {
		Group struct {
			CatalogueNumber string `json:"catalogueNumber"`
			CategoryID      int    `json:"categoryId"`
			CategoryName    string `json:"categoryName"`
			ID              int    `json:"id"`
			IsBookmarked    bool   `json:"isBookmarked"`
			MusicInfo       struct {
				Artists []struct {
					ID   int    `json:"id"`
					Name string `json:"name"`
				} `json:"artists"`
				Composers []struct {
					ID   int    `json:"id"`
					Name string `json:"name"`
				} `json:"composers"`
				Conductor []struct {
					ID   int    `json:"id"`
					Name string `json:"name"`
				} `json:"conductor"`
				Dj []struct {
					ID   int    `json:"id"`
					Name string `json:"name"`
				} `json:"dj"`
				Producer []struct {
					ID   int    `json:"id"`
					Name string `json:"name"`
				} `json:"producer"`
				RemixedBy []struct {
					ID   int    `json:"id"`
					Name string `json:"name"`
				} `json:"remixedBy"`
				With []struct {
					ID   int    `json:"id"`
					Name string `json:"name"`
				} `json:"with"`
			} `json:"musicInfo"`
			Name        string   `json:"name"`
			RecordLabel string   `json:"recordLabel"`
			ReleaseType int      `json:"releaseType"`
			Tags        []string `json:"tags"`
			Time        string   `json:"time"`
			VanityHouse bool     `json:"vanityHouse"`
			WikiBody    string   `json:"wikiBody"`
			WikiImage   string   `json:"wikiImage"`
			Year        int      `json:"year"`
		} `json:"group"`
		Torrents []struct {
			Description             string `json:"description"`
			Encoding                string `json:"encoding"`
			FileCount               int    `json:"fileCount"`
			FileList                string `json:"fileList"`
			FilePath                string `json:"filePath"`
			Format                  string `json:"format"`
			FreeTorrent             bool   `json:"freeTorrent"`
			HasCue                  bool   `json:"hasCue"`
			HasLog                  bool   `json:"hasLog"`
			ID                      int    `json:"id"`
			Leechers                int    `json:"leechers"`
			LogScore                int    `json:"logScore"`
			Media                   string `json:"media"`
			RemasterCatalogueNumber string `json:"remasterCatalogueNumber"`
			RemasterRecordLabel     string `json:"remasterRecordLabel"`
			RemasterTitle           string `json:"remasterTitle"`
			RemasterYear            int    `json:"remasterYear"`
			Remastered              bool   `json:"remastered"`
			Reported                bool   `json:"reported"`
			Scene                   bool   `json:"scene"`
			Seeders                 int    `json:"seeders"`
			Size                    int    `json:"size"`
			Snatched                int    `json:"snatched"`
			Time                    string `json:"time"`
			UserID                  int    `json:"userId"`
			Username                string `json:"username"`
		} `json:"torrents"`
	} `json:"response"`
	Status string `json:"status"`
}

type GazelleArtist struct {
	Response struct {
		Body                 string `json:"body"`
		HasBookmarked        bool   `json:"hasBookmarked"`
		ID                   int    `json:"id"`
		Image                string `json:"image"`
		Name                 string `json:"name"`
		NotificationsEnabled bool   `json:"notificationsEnabled"`
		Requests             []struct {
			Bounty     int    `json:"bounty"`
			CategoryID int    `json:"categoryId"`
			RequestID  int    `json:"requestId"`
			TimeAdded  string `json:"timeAdded"`
			Title      string `json:"title"`
			Votes      int    `json:"votes"`
			Year       int    `json:"year"`
		} `json:"requests"`
		SimilarArtists []struct {
			ArtistID  int    `json:"artistId"`
			Name      string `json:"name"`
			Score     int    `json:"score"`
			SimilarID int    `json:"similarId"`
		} `json:"similarArtists"`
		Statistics struct {
			NumGroups   int `json:"numGroups"`
			NumLeechers int `json:"numLeechers"`
			NumSeeders  int `json:"numSeeders"`
			NumSnatches int `json:"numSnatches"`
			NumTorrents int `json:"numTorrents"`
		} `json:"statistics"`
		Tags []struct {
			Count int    `json:"count"`
			Name  string `json:"name"`
		} `json:"tags"`
		Torrentgroup []struct {
			Artists []struct {
				Aliasid int    `json:"aliasid"`
				ID      int    `json:"id"`
				Name    string `json:"name"`
			} `json:"artists"`
			ExtendedArtists struct {
				One []struct {
					Aliasid int    `json:"aliasid"`
					ID      int    `json:"id"`
					Name    string `json:"name"`
				} `json:"1"`
				Two []struct {
					Aliasid int    `json:"aliasid"`
					ID      int    `json:"id"`
					Name    string `json:"name"`
				} `json:"2"`
				Three []struct {
					Aliasid int    `json:"aliasid"`
					ID      int    `json:"id"`
					Name    string `json:"name"`
				} `json:"3"`
				Four []struct {
					Aliasid int    `json:"aliasid"`
					ID      int    `json:"id"`
					Name    string `json:"name"`
				} `json:"4"`
				Five []struct {
					Aliasid int    `json:"aliasid"`
					ID      int    `json:"id"`
					Name    string `json:"name"`
				} `json:"5"`
				Six []struct {
					Aliasid int    `json:"aliasid"`
					ID      int    `json:"id"`
					Name    string `json:"name"`
				} `json:"6"`
				Seven []struct {
					Aliasid int    `json:"aliasid"`
					ID      int    `json:"id"`
					Name    string `json:"name"`
				} `json:"7"`
			} `json:"extendedArtists"`
			GroupCatalogueNumber string   `json:"groupCatalogueNumber"`
			GroupCategoryID      string   `json:"groupCategoryID"`
			GroupID              int      `json:"groupId"`
			GroupName            string   `json:"groupName"`
			GroupRecordLabel     string   `json:"groupRecordLabel"`
			GroupVanityHouse     bool     `json:"groupVanityHouse"`
			GroupYear            int      `json:"groupYear"`
			HasBookmarked        bool     `json:"hasBookmarked"`
			ReleaseType          int      `json:"releaseType"`
			Tags                 []string `json:"tags"`
			Torrent              []struct {
				Encoding            string `json:"encoding"`
				FileCount           int    `json:"fileCount"`
				Format              string `json:"format"`
				FreeTorrent         bool   `json:"freeTorrent"`
				GroupID             int    `json:"groupId"`
				HasCue              bool   `json:"hasCue"`
				HasFile             int    `json:"hasFile"`
				HasLog              bool   `json:"hasLog"`
				ID                  int    `json:"id"`
				Leechers            int    `json:"leechers"`
				LogScore            int    `json:"logScore"`
				Media               string `json:"media"`
				RemasterRecordLabel string `json:"remasterRecordLabel"`
				RemasterTitle       string `json:"remasterTitle"`
				RemasterYear        int    `json:"remasterYear"`
				Remastered          bool   `json:"remastered"`
				Scene               bool   `json:"scene"`
				Seeders             int    `json:"seeders"`
				Size                int    `json:"size"`
				Snatched            int    `json:"snatched"`
				Time                string `json:"time"`
			} `json:"torrent"`
			WikiImage string `json:"wikiImage"`
		} `json:"torrentgroup"`
		VanityHouse bool `json:"vanityHouse"`
	} `json:"response"`
	Status string `json:"status"`
}
