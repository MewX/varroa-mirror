package main

const (
	varroa        = "varroa musica"
	varroaVersion = "varroa musica -- v12dev."
	pidFile       = "varroa_pid"
	envPassphrase = "_VARROA_PASSPHRASE"

	// directories & files
	statsDir                  = "stats"
	metadataDir               = "TrackerMetadata"
	userMetadataJSONFile      = "user_metadata.json"
	originJSONFile            = "origin.json"
	trackerMetadataFile       = "Release.json"
	trackerTGroupMetadataFile = "ReleaseGroup.json"
	trackerCoverFile          = "Cover"
	summaryFile               = "Release.md"

	// file extensions
	yamlExt      = ".yaml"
	encryptedExt = ".enc"
	pngExt       = ".png"
	svgExt       = ".svg"
	csvExt       = ".csv"
	msgpackExt   = ".db"
	jsonExt      = ".json"

	// information
	infoUserFilesArchived         = "User files backed up."
	infoUsage                     = "Before running a command that requires the daemon, run 'varroa start'."
	infoEncrypted                 = "Configuration file encrypted. You can use this encrypted version in place of the unencrypted version."
	infoDecrypted                 = "Configuration file has been decrypted to a plaintext YAML file."
	infoNotInteresting            = "No filter is interested in release: %s. Ignoring."
	infoNotMusic                  = "Not a music release, ignoring."
	infoNotSnatchingDuplicate     = "Similar release already downloaded, and duplicates are not allowed"
	infoNotSnatchingUniqueInGroup = "Release from the same torrentgroup already downloaded, and snatch must be unique in group"
	infoAllMetadataSaved          = "All metadata saved."
	infoMetadataSaved             = "Metadata saved to: "
	infoArtistMetadataSaved       = "Artist Metadata for %s saved to: %s"
	infoTorrentGroupMetadataSaved = "Torrent Group Metadata for %s saved to: %s"
	infoCoverSaved                = "Cover saved to: "
	webServerNotConfigured        = "No configuration found for the web server."
	webServerShutDown             = "Web server has closed."
	webServerUpHTTP               = "Starting http web server."
	webServerUpHTTPS              = "Starting https web server."
	webServersUp                  = "Web server(s) started."

	// cli errors
	errorArguments        = "Error parsing command line arguments"
	errorInfoBadArguments = "Bad arguments"
	// daemon errors
	errorServingSignals         = "Error serving signals"
	errorFindingDaemon          = "Error finding daemon"
	errorReleasingDaemon        = "Error releasing daemon"
	errorSendingSignal          = "Error sending signal to the daemon"
	errorGettingDaemonContext   = "Error launching daemon"
	errorSendingCommandToDaemon = "Error sending command to daemon"
	errorRemovingPID            = "Error removing pid file"
	// unix socket errors
	errorDialingSocket     = "Error dialing to unix socket"
	errorWritingToSocket   = "Error writing to unix socket"
	errorReadingFromSocket = "Error reading from unix socket"
	errorCreatingSocket    = "Error creating unix socket"
	// command reload errors
	errorReloading = "Error reloading"
	// command check-log errors
	errorCheckingLog     = "Error checking log"
	errorGettingLogScore = "Error getting log score"
	// command snatch erros
	errorSnatchingTorrent = "Error snatching torrent"
	// command refresh-metadata errors
	errorRefreshingMetadata = "Error refreshing metadata"
	errorCannotFindID       = "Error with ID#%s, not found in history or in downloads directory."
	// command backup errors
	errorArchiving = "Error while archiving user files"
	// set up errors
	errorCreatingStatsDir   = "Error creating stats directory"
	errorSettingUp          = "Error setting up"
	errorLoadingConfig      = "Error loading configuration"
	errorGettingPassphrase  = "Error getting passphrase"
	errorPassphraseNotFound = "Error retrieving passphrase for daemon"
	errorSettingEnv         = "Could not set env variable"
	// webserver errors
	errorShuttingDownServer      = "Error shutting down web server"
	errorServing                 = "Error launching web interface"
	errorWrongToken              = "Error receiving download order from https: wrong token"
	errorNoToken                 = "Error receiving download order from https: no token"
	errorNoID                    = "Error retreiving torrent ID"
	errorNoStatsFilename         = "Error retreiving stats filename "
	errorUnknownCommand          = "Error: unknown websocket command: "
	errorIncomingWebSocketJSON   = "Error parsing websocket input"
	errorIncorrectWebServerToken = "Error validating token for web server, ignoring."
	errorWritingToWebSocket      = "Error writing to websocket"
	errorCreatingWebSocket       = "Error creating websocket"
	// certificates errors
	errorOpenSSL               = "openssl is not available on this system. "
	errorGeneratingCertificate = "Error generating self-signed certificate"
	errorCreatingCertDir       = "Error creating certificates directory"
	errorCreatingFile          = "Error creating file in certificates directory"
	// crypto errors
	errorBadPassphrase        = "Error, passphrase must be 32bytes long."
	errorCanOnlyEncryptYAML   = "Error encrypting, input is not a .yaml file."
	errorCanOnlyDencryptENC   = "Error decrypting, input is not a .enc file."
	errorBadDecryptedFile     = "Decrypted file is not a valid YAML file (bad passphrase?)"
	errorReadingDecryptedFile = "Decrypted configuration file makes no sense."
	// graphs errors
	errorImageNotFound = "Error opening png"
	errorNoImageFound  = "Error: no image found"
	// history errors
	errorLoadingLine       = "Error loading line %d of history file"
	errorNoHistory         = "No history yet"
	errorInvalidTimestamp  = "Error parsing timestamp"
	errorNoFurtherSnatches = "No additional snatches since last time, not regenerating daily graphs."
	errorNotEnoughDays     = "Not enough days in history to generate daily graphs"
	errorMovingFile        = "Error moving file to stats folder"
	errorMigratingFile     = "Error migrating file to latest format"
	errorCreatingGraphs    = "Error, could not generate any graph."
	errorGeneratingGraph   = "Error generating graph"
	// git errors
	errorGitInit      = "Error running git init"
	errorGitAdd       = "Error running git add"
	errorGitCommit    = "Error running git commit"
	errorGitAddRemote = "Error running git remote add"
	errorGitPush      = "Error running git push"
	// irc errors
	errorDealingWithAnnounce    = "Error dealing with announced torrent"
	errorConnectingToIRC        = "Error connecting to IRC"
	errorCouldNotGetTorrentInfo = "Error retreiving torrent info from tracker"
	errorCouldNotMoveTorrent    = "Error moving torrent to destination folder"
	errorDownloadingTorrent     = "Error downloading torrent"
	errorRemovingTempFile       = "Error removing temporary file %s"
	errorAddingToHistory        = "Error adding release to history"
	// notifications errors
	errorNotification = "Error while sending pushover notification"
	// release metadata errors
	errorWritingJSONMetadata        = "Error writing metadata file"
	errorDownloadingTrackerCover    = "Error downloading tracker cover"
	errorCreatingMetadataDir        = "Error creating metadata directory"
	errorRetrievingArtistInfo       = "Error getting info for artist %d"
	errorRetrievingTorrentGroupInfo = "Error getting torrent group info for %d"
	errorWithOriginJSON             = "Error creating or updating origin.json"
	errorInfoNoMatchForOrigin       = "Error updating origin.json, no match for tracker and/or torrent ID"
	errorGeneratingUserMetadataJSON = "Error generating user metadata JSON"
	errorGeneratingSummary          = "Error generating metadata summary"
	// stats errors
	errorGettingStats          = "Error getting stats"
	errorWritingCSV            = "Error writing stats to CSV file"
	errorGeneratingGraphs      = "Error generating graphs (may require more data)"
	errorGeneratingDailyGraphs = "Error generating daily graphs (at least 24h worth of data required): "
	errorNotEnoughDataPoints   = "Not enough data points (yet) to generate graph"
	errorBufferDrop            = "Buffer drop too important, stopping autosnatching. Reload to start again."
	// tracker errors
	errorUnknownTorrentURL        = "Unknown torrent URL"
	errorLogIn                    = "Error logging in"
	errorNotLoggedIn              = "Not logged in"
	errorJSONAPI                  = "Error calling JSON API"
	errorGET                      = "Error calling GET on URL, got HTTP status: "
	errorUnmarshallingJSON        = "Error reading JSON"
	errorInvalidResponse          = "Invalid response. Maybe log in again?"
	errorAPIResponseStatus        = "Got JSON API status: "
	errorCouldNotCreateForm       = "Could not create form for log"
	errorCouldNotReadLog          = "Could not read log"
	errorGazelleRateLimitExceeded = "rate limit exceeded"
)
