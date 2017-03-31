package main

type GazelleGenericResponse struct {
	Response interface{} `json:"response"`
	Status   string      `json:"status"`
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
			Name        string `json:"name"`
			RecordLabel string `json:"recordLabel"`
			ReleaseType int    `json:"releaseType"`
			Time        string `json:"time"`
			VanityHouse bool   `json:"vanityHouse"`
			WikiBody    string `json:"wikiBody"`
			WikiImage   string `json:"wikiImage"`
			Year        int    `json:"year"`
		} `json:"group"`
		Torrent struct {
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
				Two   interface{} `json:"2"`
				Three interface{} `json:"3"`
				Four  interface{} `json:"4"`
				Five  interface{} `json:"5"`
				Six   interface{} `json:"6"`
				Seven interface{} `json:"7"`
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
