package varroa

var (
	// Version will be updated by the Makefile at build time.
	Version = "dev"
)

const (
	FullName       = "varroa musica"
	FullNameAlt    = "VarroaMusica"
	FullVersion    = "%s -- %s."
	DefaultPIDFile = "varroa_pid"
	DefaultLogFile = "log"
	envPassphrase  = "_VARROA_PASSPHRASE"

	// directories & files
	DefaultConfigurationFile   = "config.yaml"
	daemonSocket               = "varroa.sock"
	StatsDir                   = "stats"
	MetadataDir                = "TrackerMetadata"
	downloadsCleanDir          = "VarroaClean"
	userMetadataJSONFile       = "user_metadata.json"
	OriginJSONFile             = "origin.json"
	trackerMetadataFile        = "Release.json"
	trackerTGroupMetadataFile  = "Group.json"
	trackerCollageMetadataFile = "%s collage #%d.json"
	trackerCoverFile           = "Cover"
	perDay                     = "per_day_"
	uploadStatsFile            = "up"
	downloadStatsFile          = "down"
	ratioStatsFile             = "ratio"
	bufferStatsFile            = "buffer"
	warningBufferStatsFile     = "warningbuffer"
	overallStatsFile           = "stats"
	numberSnatchedPerDayFile   = "snatches_per_day"
	sizeSnatchedPerDayFile     = "size_snatched_per_day"
	totalSnatchesByFilterFile  = "total_snatched_by_filter"
	toptagsFile                = "top_tags"
	gitlabCIYamlFile           = ".gitlab-ci.yml"
	htmlIndexFile              = "index.html"
	defaultFolderTemplate      = "$a ($y) $t {$id} [$f $s]"
	DefaultHistoryDB           = "history.db"
	DefaultDownloadsDB         = "downloads.db"
	DefaultLibraryDB           = "library.db"
	manualSnatchFilterName     = "remote"
	overallPrefix              = "overall"
	lastWeekPrefix             = "lastweek"
	lastMonthPrefix            = "lastmonth"
	statsNotificationPrefix    = "stats: "

	// Notable ratios & constants
	defaultTargetRatio = 1.0
	warningRatio       = 0.6
	minimumSeeders     = 5

	// file extensions
	yamlExt      = ".yaml"
	encryptedExt = ".enc"
	pngExt       = ".png"
	svgExt       = ".svg"
	msgpackExt   = ".db"
	jsonExt      = ".json"
	m3uExt       = ".m3u"

	// filters
	filterRegExpPrefix        = "r/"
	filterExcludeRegExpPrefix = "xr/"

	// information
	InfoUserFilesArchived         = "User files backed up."
	InfoUsage                     = "Before running a command that requires the daemon, run 'varroa start'."
	InfoEncrypted                 = "Configuration file encrypted. You can use this encrypted version in place of the unencrypted version."
	InfoDecrypted                 = "Configuration file has been decrypted to a plaintext YAML file."
	infoNotInteresting            = "No filter is interested in release: %s. Ignoring."
	infoNotMusic                  = "Not a music release, ignoring."
	infoNotSnatchingDuplicate     = "Similar release already downloaded, and duplicates are not allowed"
	infoFilterIgnoredForTracker   = "Filter %s ignored for tracker %s."
	infoFilterTriggered           = "This release would trigger filter %s!"
	infoNotSnatchingUniqueInGroup = "Release from the same torrentgroup already downloaded, and snatch must be unique in group"
	infoAllMetadataSaved          = "All %s metadata saved to: %s."
	infoAllMetadataSaving         = "Saving metadata to: %s."
	infoMetadataSaved             = "Release metadata saved."
	infoArtistMetadataSaved       = "%s artist metadata %s saved."
	infoTorrentGroupMetadataSaved = "Torrent Group metadata saved."
	infoCollageMetadataSaved      = "Collage #%d metadata saved."
	infoCoverSaved                = "Cover saved."
	webServerNotConfigured        = "No configuration found for the web server."
	webServerShutDown             = "Web server has closed."
	webServerUpHTTP               = "Starting http web server."
	webServerUpHTTPS              = "Starting https web server."
	webServersUp                  = "Web server(s) started."

	// cli errors
	ErrorArguments        = "Error parsing command line arguments"
	ErrorInfoBadArguments = "Bad arguments"
	// daemon errors
	ErrorFindingDaemon          = "Error finding daemon"
	ErrorGettingDaemonContext   = "Error launching daemon (it probably is running already)"
	ErrorSendingCommandToDaemon = "Error sending command to daemon"
	// command check-log errors
	ErrorCheckingLog     = "Error checking log"
	errorGettingLogScore = "Error getting log score"
	// command snatch errors
	ErrorSnatchingTorrent = "Error snatching torrent"
	// command info errors
	ErrorShowingTorrentInfo = "Error displaying torrent info"
	// command refresh-metadata errors
	ErrorRefreshingMetadata = "Error refreshing metadata"
	errorCannotFindID       = "Error with ID#%s, not found in history or in downloads directory."
	// command reseed
	ErrorReseed = "error trying to reseed release"
	// command backup errors
	errorArchiving = "Error while archiving user files"
	// set up errors
	errorCreatingStatsDir          = "Error creating stats directory"
	errorCreatingDownloadsCleanDir = "Error creating directory for useless folders in downloads directory"
	ErrorSettingUp                 = "Error setting up"
	ErrorLoadingConfig             = "Error loading configuration"
	errorReadingConfig             = "Error reading configuration file"
	errorLoadingYAML               = "YAML file cannot be parsed, check if it is correctly formatted and has all the required parts"
	errorGettingPassphrase         = "Error getting passphrase"
	errorPassphraseNotFound        = "Error retrieving passphrase for daemon"
	errorSettingEnv                = "Could not set env variable"
	// webserver errors
	errorServing                 = "Error launching web interface"
	errorWrongToken              = "Error receiving download order from https: wrong token"
	errorNoToken                 = "Error receiving download order from https: no token"
	errorNoID                    = "Error retrieving torrent ID"
	errorNoStatsFilename         = "Error retrieving stats filename "
	errorUnknownCommand          = "Error: unknown websocket command: "
	errorIncomingWebSocketJSON   = "Error parsing websocket input"
	errorOutgoingWebSocketJSON   = "Error writing to websocket"
	errorIncorrectWebServerToken = "Error validating token for web server, ignoring."
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
	errorGeneratingGraph = "Error generating graph"
	// git errors
	errorGitInit      = "Error running git init"
	errorGitAdd       = "Error running git add"
	errorGitCommit    = "Error running git commit"
	errorGitAddRemote = "Error running git remote add"
	errorDeploying    = "Error deploying to Gitlab Pages"
	// irc errors
	errorDealingWithAnnounce    = "Error dealing with announced torrent"
	errorConnectingToIRC        = "Error connecting to IRC"
	errorCouldNotGetTorrentInfo = "Error retrieving torrent info from tracker"
	errorDownloadingTorrent     = "Error downloading torrent"
	errorAddingToHistory        = "Error adding release to history"
	announcerBadCredentials     = "Bad credentials."
	// notifications errors
	errorNotification  = "Error while sending pushover notification"
	errorWebhook       = "Error pushing webhook POST"
	errorNotifications = "Error while sending notifications"
	// release metadata errors
	errorWritingJSONMetadata        = "Error writing metadata file"
	errorDownloadingTrackerCover    = "Error downloading tracker cover"
	errorCreatingMetadataDir        = "Error creating metadata directory"
	errorRetrievingArtistInfo       = "Error getting info for artist %d"
	errorRetrievingTorrentGroupInfo = "Error getting torrent group info for %d"
	errorRetrievingCollageInfo      = "Error getting collage info for %d"
	errorWithOriginJSON             = "Error creating or updating origin.json"
	errorGeneratingUserMetadataJSON = "Error generating user metadata JSON"
	ErrorFindingMusicAndMetadata    = "directory %s does not contain music files and tracker metadata"
	couldNotFindMetadataAge         = "No information about metadata age found."
	// stats errors
	errorGettingStats      = "Error getting stats"
	ErrorGeneratingGraphs  = "Error generating graphs (may require more data, 24h worth for daily graphs)"
	errorBufferDrop        = "Buffer drop too important, stopping autosnatching. Restart to start again."
	errorBelowWarningRatio = "Ratio below warning level, stopping autosnatching."

	// downloads db errors
	errorCleaningDownloads = "Error cleaning up download: "
	// disk space usage
	currentUsage     = "Current disk usage: %.2f%% used, remaining: %s"
	lowDiskSpace     = "Warning: low disk space available (<5%)"
	veryLowDiskSpace = "Warning: very low disk space available (<2%)"

	// generic constants
	scanningFiles = "Scanning"
)

func userAgent() string {
	return FullNameAlt + "/" + Version[1:]
}
