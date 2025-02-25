package qbittorrent

import (
	"strings"

	"github.com/sagan/ptool/client"
	"github.com/sagan/ptool/util"
)

type apiTorrentTracker struct {
	Url string `yaml:"url"` // Tracker url
	// Tracker status.
	// 0	Tracker is disabled (used for DHT, PeX, and LSD);
	// 1	Tracker has not been contacted yet;
	// 2	Tracker has been contacted and is working;
	// 3	Tracker is updating;
	// 4	Tracker has been contacted, but it is not working (or doesn't send proper replies);
	Status         int64  `yaml:"status"`
	Tier           int64  `yaml:"tier"`           // Tracker priority tier. Lower tier trackers are tried before higher tiers. Tier numbers are valid when >= 0, < 0 is used as placeholder when tier does not exist for special entries (such as DHT).
	Num_peers      int64  `yaml:"num_peers"`      // Number of peers for current torrent, as reported by the tracker
	Num_seeds      int64  `yaml:"num_seeds"`      // Number of seeds for current torrent, as reported by the tracker
	Num_leeches    int64  `yaml:"num_leeches"`    // Number of leeches for current torrent, as reported by the tracker
	Num_downloaded int64  `yaml:"num_downloaded"` // Number of completed downlods for current torrent, as reported by the tracker
	Msg            string `yaml:"msg"`            // Tracker message (there is no way of knowing what this message is - it's up to tracker admins)
}

type apiTorrentContent struct {
	Index        int64   `json:"index"`        // File index
	Name         string  `json:"name"`         // File name (including relative path)
	Size         int64   `json:"size"`         // File size (bytes)
	Progress     float64 `json:"progress"`     // File progress (percentage/100)
	Priority     int64   `json:"priority"`     // File priority. 0: Do not download. 7 (max), 6(high), 1(normal) prio
	Is_seed      bool    `json:"is_seed"`      // True if file is seeding/complete
	Piece_range  []int64 `json:"piece_range"`  // array	The first number is the starting piece index and the second number is the ending piece index (inclusive)
	Availability float64 `json:"availability"` // Percentage of file pieces currently available (percentage/100)
}

type apiSyncMaindata struct {
	Server_state *apiTransferInfo                   `json:"server_state"`
	Tags         []string                           `json:"tags"`
	Categories   map[string]*client.TorrentCategory `json:"categories"`
	Torrents     map[string]*apiTorrentInfo         `json:"torrents"`
}

type apiTransferInfo struct {
	Free_space_on_disk int64  `json:"free_space_on_disk"`
	Dl_info_speed      int64  `json:"dl_info_speed"`     //Global download rate (bytes/s)
	Dl_info_data       int64  `json:"dl_info_data"`      //Data downloaded this session (bytes)
	Up_info_speed      int64  `json:"up_info_speed"`     //Global upload rate (bytes/s)
	Up_info_data       int64  `json:"up_info_data"`      //Data uploaded this session (bytes)
	Dl_rate_limit      int64  `json:"dl_rate_limit"`     //Download rate limit (bytes/s)
	Up_rate_limit      int64  `json:"up_rate_limit"`     //Upload rate limit (bytes/s)
	Dht_nodes          int64  `json:"dht_nodes"`         //DHT nodes connected to
	Connection_status  string `json:"connection_status"` //Connection status. connected|firewalled|disconnected
}

type apiTorrentProperties struct {
	Save_path                string  `json:"save_path"`                // Torrent save path
	Creation_date            int64   `json:"creation_date"`            // Torrent creation date (Unix timestamp)
	Piece_size               int64   `json:"piece_size"`               // Torrent piece size (bytes)
	Comment                  string  `json:"comment"`                  // Torrent comment
	Total_wasted             int64   `json:"total_wasted"`             // Total data wasted for torrent (bytes)
	Total_uploaded           int64   `json:"total_uploaded"`           // Total data uploaded for torrent (bytes)
	Total_uploaded_session   int64   `json:"total_uploaded_session"`   // Total data uploaded this session (bytes)
	Total_downloaded         int64   `json:"total_downloaded"`         // Total data downloaded for torrent (bytes)
	Total_downloaded_session int64   `json:"total_downloaded_session"` // Total data downloaded this session (bytes)
	Up_limit                 int64   `json:"up_limit"`                 // Torrent upload limit (bytes/s)
	Dl_limit                 int64   `json:"dl_limit"`                 // Torrent download limit (bytes/s)
	Time_elapsed             int64   `json:"time_elapsed"`             // Torrent elapsed time (seconds)
	Seeding_time             int64   `json:"seeding_time"`             // Torrent elapsed time while complete (seconds)
	Nb_connections           int64   `json:"nb_connections"`           // Torrent connection count
	Nb_connections_limit     int64   `json:"nb_connections_limit"`     // Torrent connection count limit
	Share_ratio              float64 `json:"share_ratio"`              // Torrent share ratio
	Addition_date            int64   `json:"addition_date"`            // When this torrent was added (unix timestamp)
	Completion_date          int64   `json:"completion_date"`          // Torrent completion date (unix timestamp)
	Created_by               string  `json:"created_by"`               // Torrent creator
	Dl_speed_avg             int64   `json:"dl_speed_avg"`             // Torrent average download speed (bytes/second)
	Dl_speed                 int64   `json:"dl_speed"`                 // Torrent download speed (bytes/second)
	Eta                      int64   `json:"eta"`                      // Torrent ETA (seconds)
	Last_seen                int64   `json:"last_seen"`                // Last seen complete date (unix timestamp)
	Peers                    int64   `json:"peers"`                    // Number of peers connected to
	Peers_total              int64   `json:"peers_total"`              // Number of peers in the swarm
	Pieces_have              int64   `json:"pieces_have"`              // Number of pieces owned
	Pieces_num               int64   `json:"pieces_num"`               // Number of pieces of the torrent
	Reannounce               int64   `json:"reannounce"`               // Number of seconds until the next announce
	Seeds                    int64   `json:"seeds"`                    // Number of seeds connected to
	Seeds_total              int64   `json:"seeds_total"`              // Number of seeds in the swarm
	Total_size               int64   `json:"total_size"`               // Torrent total size (bytes)
	Up_speed_avg             int64   `json:"up_speed_avg"`             // Torrent average upload speed (bytes/second)
	Up_speed                 int64   `json:"up_speed"`                 // Torrent upload speed (bytes/second)
}

type apiTorrentInfo struct {
	Added_on           int64   `json:"added_on"`           //	integer	Time (Unix Epoch) when the torrent was added to the client
	Amount_left        int64   `json:"amount_left"`        //	integer	Amount of data left to download (bytes)
	Auto_tmm           bool    `json:"auto_tmm"`           //	bool	Whether this torrent is managed by Automatic Torrent Management
	Availability       float64 `json:"availability"`       //	float	Percentage of file pieces currently available
	Category           string  `json:"category"`           //	string	Category of the torrent
	Completed          int64   `json:"completed"`          //	integer	Amount of transfer data completed (bytes)
	Completion_on      int64   `json:"completion_on"`      //	integer	Time (Unix Epoch) when the torrent completed
	Content_path       string  `json:"content_path"`       //	string	Absolute path of torrent content (root path for multifile torrents; absolute file path for singlefile torrents)
	Dl_limit           int64   `json:"dl_limit"`           //	integer	Torrent download speed limit (bytes/s). -1 if ulimited.
	Dlspeed            int64   `json:"dlspeed"`            //	integer	Torrent download speed (bytes/s)
	Downloaded         int64   `json:"downloaded"`         //	integer	Amount of data downloaded
	Downloaded_session int64   `json:"downloaded_session"` //	integer	Amount of data downloaded this session
	Eta                int64   `json:"eta"`                //	integer	Torrent ETA (seconds)
	F_l_piece_prio     bool    `json:"f_l_piece_prio"`     //	bool	True if first last piece are prioritized
	Force_start        bool    `json:"force_start"`        //	bool	True if force start is enabled for this torrent
	Hash               string  `json:"hash"`               //	string	Torrent hash
	Last_activity      int64   `json:"last_activity"`      //	integer	Last time (Unix Epoch) when a chunk was downloaded/uploaded
	Magnet_uri         string  `json:"magnet_uri"`         //	string	Magnet URI corresponding to this torrent
	Max_ratio          float64 `json:"max_ratio"`          //	float	Maximum share ratio until torrent is stopped from seeding/uploading
	Max_seeding_time   int64   `json:"max_seeding_time"`   //	integer	Maximum seeding time (seconds) until torrent is stopped from seeding
	Name               string  `json:"name"`               //	string	Torrent name
	Num_complete       int64   `json:"num_complete"`       //	integer	Number of seeds in the swarm
	Num_incomplete     int64   `json:"num_incomplete"`     //	integer	Number of leechers in the swarm
	Num_leechs         int64   `json:"num_leechs"`         //	integer	Number of leechers connected to
	Num_seeds          int64   `json:"num_seeds"`          //	integer	Number of seeds connected to
	Priority           int64   `json:"priority"`           //	integer	Torrent priority. Returns -1 if queuing is disabled or torrent is in seed mode
	Progress           float64 `json:"progress"`           //	float	Torrent progress (percentage/100)
	Ratio              float64 `json:"ratio"`              //	float	Torrent share ratio. Max ratio value: 9999.
	Ratio_limit        float64 `json:"ratio_limit"`        //	float	TODO (what is different from max_ratio?)
	Save_path          string  `json:"save_path"`          //	string	Path where this torrent's data is stored
	Seeding_time       int64   `json:"seeding_time"`       //	integer	Torrent elapsed time while complete (seconds)
	Seeding_time_limit int64   `json:"seeding_time_limit"` //	integer	TODO (what is different from max_seeding_time?) seeding_time_limit is a per torrent setting when Automatic Torrent Management is disabled furthermore then max_seeding_time is set to seeding_time_limit for this torrent. If Automatic Torrent Management is enabled the value is -2. And if max_seeding_time is unset it have a default value -1.
	Seen_complete      int64   `json:"seen_complete"`      //	integer	Time (Unix Epoch) when this torrent was last seen complete
	Seq_dl             bool    `json:"seq_dl"`             //	bool	True if sequential download is enabled
	Size               int64   `json:"size"`               //	integer	Total size (bytes) of files selected for download
	State              string  `json:"state"`              //	string	Torrent state. See table here below for the possible values
	Super_seeding      bool    `json:"super_seeding"`      //	bool	True if super seeding is enabled
	Tags               string  `json:"tags"`               //	string	Comma-concatenated tag list of the torrent
	Time_active        int64   `json:"time_active"`        //	integer	Total active time (seconds)
	Total_size         int64   `json:"total_size"`         //	integer	Total size (bytes) of all file in this torrent (including unselected ones)
	Tracker            string  `json:"tracker"`            //	string	The first tracker with working status. Returns empty string if no tracker is working.
	Up_limit           int64   `json:"up_limit"`           //	integer	Torrent upload speed limit (bytes/s). -1 if ulimited.
	Uploaded           int64   `json:"uploaded"`           //	integer	Amount of data uploaded
	Uploaded_session   int64   `json:"uploaded_session"`   //	integer	Amount of data uploaded this session
	Upspeed            int64   `json:"upspeed"`            //	integer	Torrent upload speed (bytes/s)
}

type apiPreferences struct {
	Locale                                 string         `json:"locale"`                                 // Currently selected language (e.g. en_GB for English)
	Create_subfolder_enabled               bool           `json:"create_subfolder_enabled"`               // True if a subfolder should be created when adding a torrent
	Start_paused_enabled                   bool           `json:"start_paused_enabled"`                   // True if torrents should be added in a Paused state
	Auto_delete_mode                       int64          `json:"auto_delete_mode"`                       // TODO
	Preallocate_all                        bool           `json:"preallocate_all"`                        // True if disk space should be pre-allocated for all files
	Incomplete_files_ext                   bool           `json:"incomplete_files_ext"`                   // True if ".!qB" should be appended to incomplete files
	Auto_tmm_enabled                       bool           `json:"auto_tmm_enabled"`                       // True if Automatic Torrent Management is enabled by default
	Torrent_changed_tmm_enabled            bool           `json:"torrent_changed_tmm_enabled"`            // True if torrent should be relocated when its Category changes
	Save_path_changed_tmm_enabled          bool           `json:"save_path_changed_tmm_enabled"`          // True if torrent should be relocated when the default save path changes
	Category_changed_tmm_enabled           bool           `json:"category_changed_tmm_enabled"`           // True if torrent should be relocated when its Category's save path changes
	Save_path                              string         `json:"save_path"`                              // Default save path for torrents, separated by slashes
	Temp_path_enabled                      bool           `json:"temp_path_enabled"`                      // True if folder for incomplete torrents is enabled
	Temp_path                              string         `json:"temp_path"`                              // Path for incomplete torrents, separated by slashes
	Scan_dirs                              map[string]any `json:"scan_dirs"`                              // Property: directory to watch for torrent files, value: where torrents loaded from this directory should be downloaded to (see list of possible values below). Slashes are used as path separators; multiple key/value pairs can be specified
	Export_dir                             string         `json:"export_dir"`                             // Path to directory to copy .torrent files to. Slashes are used as path separators
	Export_dir_fin                         string         `json:"export_dir_fin"`                         // Path to directory to copy .torrent files of completed downloads to. Slashes are used as path separators
	Mail_notification_enabled              bool           `json:"mail_notification_enabled"`              // True if e-mail notification should be enabled
	Mail_notification_sender               string         `json:"mail_notification_sender"`               // e-mail where notifications should originate from
	Mail_notification_email                string         `json:"mail_notification_email"`                // e-mail to send notifications to
	Mail_notification_smtp                 string         `json:"mail_notification_smtp"`                 // smtp server for e-mail notifications
	Mail_notification_ssl_enabled          bool           `json:"mail_notification_ssl_enabled"`          // True if smtp server requires SSL connection
	Mail_notification_auth_enabled         bool           `json:"mail_notification_auth_enabled"`         // True if smtp server requires authentication
	Mail_notification_username             string         `json:"mail_notification_username"`             // Username for smtp authentication
	Mail_notification_password             string         `json:"mail_notification_password"`             // Password for smtp authentication
	Autorun_enabled                        bool           `json:"autorun_enabled"`                        // True if external program should be run after torrent has finished downloading
	Autorun_program                        string         `json:"autorun_program"`                        // Program path/name/arguments to run if autorun_enabled is enabled; path is separated by slashes; you can use %f and %n arguments, which will be expanded by qBittorent as path_to_torrent_file and torrent_name (from the GUI; not the .torrent file name) respectively
	Queueing_enabled                       bool           `json:"queueing_enabled"`                       // True if torrent queuing is enabled
	Max_active_downloads                   int64          `json:"max_active_downloads"`                   // Maximum number of active simultaneous downloads
	Max_active_torrents                    int64          `json:"max_active_torrents"`                    // Maximum number of active simultaneous downloads and uploads
	Max_active_uploads                     int64          `json:"max_active_uploads"`                     // Maximum number of active simultaneous uploads
	Dont_count_slow_torrents               bool           `json:"dont_count_slow_torrents"`               // If true torrents w/o any activity (stalled ones) will not be counted towards max_active_* limits; see dont_count_slow_torrents for more information
	Slow_torrent_dl_rate_threshold         int64          `json:"slow_torrent_dl_rate_threshold"`         // Download rate in KiB/s for a torrent to be considered "slow"
	Slow_torrent_ul_rate_threshold         int64          `json:"slow_torrent_ul_rate_threshold"`         // Upload rate in KiB/s for a torrent to be considered "slow"
	Slow_torrent_inactive_timer            int64          `json:"slow_torrent_inactive_timer"`            // Seconds a torrent should be inactive before considered "slow"
	Max_ratio_enabled                      bool           `json:"max_ratio_enabled"`                      // True if share ratio limit is enabled
	Max_ratio                              float64        `json:"max_ratio"`                              // Get the global share ratio limit
	Max_ratio_act                          int64          `json:"max_ratio_act"`                          // Action performed when a torrent reaches the maximum share ratio. See list of possible values here below.
	Listen_port                            int64          `json:"listen_port"`                            // Port for incoming connections
	Upnp                                   bool           `json:"upnp"`                                   // True if UPnP/NAT-PMP is enabled
	Random_port                            bool           `json:"random_port"`                            // True if the port is randomly selected
	Dl_limit                               int64          `json:"dl_limit"`                               // Global download speed limit in KiB/s; -1 means no limit is applied
	Up_limit                               int64          `json:"up_limit"`                               // Global upload speed limit in KiB/s; -1 means no limit is applied
	Max_connec                             int64          `json:"max_connec"`                             // Maximum global number of simultaneous connections
	Max_connec_per_torrent                 int64          `json:"max_connec_per_torrent"`                 // Maximum number of simultaneous connections per torrent
	Max_uploads                            int64          `json:"max_uploads"`                            // Maximum number of upload slots
	Max_uploads_per_torrent                int64          `json:"max_uploads_per_torrent"`                // Maximum number of upload slots per torrent
	Stop_tracker_timeout                   int64          `json:"stop_tracker_timeout"`                   // Timeout in seconds for a stopped announce request to trackers
	Enable_piece_extent_affinity           bool           `json:"enable_piece_extent_affinity"`           // True if the advanced libtorrent option piece_extent_affinity is enabled
	Bittorrent_protocol                    int64          `json:"bittorrent_protocol"`                    // Bittorrent Protocol to use (see list of possible values below)
	Limit_utp_rate                         bool           `json:"limit_utp_rate"`                         // True if [du]l_limit should be applied to uTP connections; this option is only available in qBittorent built against libtorrent version 0.16.X and higher
	Limit_tcp_overhead                     bool           `json:"limit_tcp_overhead"`                     // True if [du]l_limit should be applied to estimated TCP overhead (service data: e.g. packet headers)
	Limit_lan_peers                        bool           `json:"limit_lan_peers"`                        // True if [du]l_limit should be applied to peers on the LAN
	Alt_dl_limit                           int64          `json:"alt_dl_limit"`                           // Alternative global download speed limit in KiB/s
	Alt_up_limit                           int64          `json:"alt_up_limit"`                           // Alternative global upload speed limit in KiB/s
	Scheduler_enabled                      bool           `json:"scheduler_enabled"`                      // True if alternative limits should be applied according to schedule
	Schedule_from_hour                     int64          `json:"schedule_from_hour"`                     // Scheduler starting hour
	Schedule_from_min                      int64          `json:"schedule_from_min"`                      // Scheduler starting minute
	Schedule_to_hour                       int64          `json:"schedule_to_hour"`                       // Scheduler ending hour
	Schedule_to_min                        int64          `json:"schedule_to_min"`                        // Scheduler ending minute
	Scheduler_days                         int64          `json:"scheduler_days"`                         // Scheduler days. See possible values here below
	Dht                                    bool           `json:"dht"`                                    // True if DHT is enabled
	Pex                                    bool           `json:"pex"`                                    // True if PeX is enabled
	Lsd                                    bool           `json:"lsd"`                                    // True if LSD is enabled
	Encryption                             int64          `json:"encryption"`                             // See list of possible values here below
	Anonymous_mode                         bool           `json:"anonymous_mode"`                         // If true anonymous mode will be enabled; read more here; this option is only available in qBittorent built against libtorrent version 0.16.X and higher
	Proxy_type                             any            `json:"proxy_type"`                             // In qb 4.x API proxy_type is a (enum) integer (-1 - 5). However, qb 5.0 changed it to string (e.g. "None"). For compatibility, set it to any.
	Proxy_ip                               string         `json:"proxy_ip"`                               // Proxy IP address or domain name
	Proxy_port                             int64          `json:"proxy_port"`                             // Proxy port
	Proxy_peer_connections                 bool           `json:"proxy_peer_connections"`                 // True if peer and web seed connections should be proxified; this option will have any effect only in qBittorent built against libtorrent version 0.16.X and higher
	Proxy_auth_enabled                     bool           `json:"proxy_auth_enabled"`                     // True proxy requires authentication; doesn't apply to SOCKS4 proxies
	Proxy_username                         string         `json:"proxy_username"`                         // Username for proxy authentication
	Proxy_password                         string         `json:"proxy_password"`                         // Password for proxy authentication
	Proxy_torrents_only                    bool           `json:"proxy_torrents_only"`                    // True if proxy is only used for torrents
	Ip_filter_enabled                      bool           `json:"ip_filter_enabled"`                      // True if external IP filter should be enabled
	Ip_filter_path                         string         `json:"ip_filter_path"`                         // Path to IP filter file (.dat, .p2p, .p2b files are supported); path is separated by slashes
	Ip_filter_trackers                     bool           `json:"ip_filter_trackers"`                     // True if IP filters are applied to trackers
	Web_ui_domain_list                     string         `json:"web_ui_domain_list"`                     // Comma-separated list of domains to accept when performing Host header validation
	Web_ui_address                         string         `json:"web_ui_address"`                         // IP address to use for the WebUI
	Web_ui_port                            int64          `json:"web_ui_port"`                            // WebUI port
	Web_ui_upnp                            bool           `json:"web_ui_upnp"`                            // True if UPnP is used for the WebUI port
	Web_ui_username                        string         `json:"web_ui_username"`                        // WebUI username
	Web_ui_password                        string         `json:"web_ui_password"`                        // For API ≥ v2.3.0: Plaintext WebUI password, not readable, write-only. For API < v2.3.0: MD5 hash of WebUI password, hash is generated from the following string: username:Web UI Access:plain_text_web_ui_password
	Web_ui_csrf_protection_enabled         bool           `json:"web_ui_csrf_protection_enabled"`         // True if WebUI CSRF protection is enabled
	Web_ui_clickjacking_protection_enabled bool           `json:"web_ui_clickjacking_protection_enabled"` // True if WebUI clickjacking protection is enabled
	Web_ui_secure_cookie_enabled           bool           `json:"web_ui_secure_cookie_enabled"`           // True if WebUI cookie Secure flag is enabled
	Web_ui_max_auth_fail_count             int64          `json:"web_ui_max_auth_fail_count"`             // Maximum number of authentication failures before WebUI access ban
	Web_ui_ban_duration                    int64          `json:"web_ui_ban_duration"`                    // WebUI access ban duration in seconds
	Web_ui_session_timeout                 int64          `json:"web_ui_session_timeout"`                 // Seconds until WebUI is automatically signed off
	Web_ui_host_header_validation_enabled  bool           `json:"web_ui_host_header_validation_enabled"`  // True if WebUI host header validation is enabled
	Bypass_local_auth                      bool           `json:"bypass_local_auth"`                      // True if authentication challenge for loopback address (127.0.0.1) should be disabled
	Bypass_auth_subnet_whitelist_enabled   bool           `json:"bypass_auth_subnet_whitelist_enabled"`   // True if webui authentication should be bypassed for clients whose ip resides within (at least) one of the subnets on the whitelist
	Bypass_auth_subnet_whitelist           string         `json:"bypass_auth_subnet_whitelist"`           // (White)list of ipv4/ipv6 subnets for which webui authentication should be bypassed; list entries are separated by commas
	Alternative_webui_enabled              bool           `json:"alternative_webui_enabled"`              // True if an alternative WebUI should be used
	Alternative_webui_path                 string         `json:"alternative_webui_path"`                 // File path to the alternative WebUI
	Use_https                              bool           `json:"use_https"`                              // True if WebUI HTTPS access is enabled
	Ssl_key                                string         `json:"ssl_key"`                                // For API < v2.0.1: SSL keyfile contents (this is a not a path)
	Ssl_cert                               string         `json:"ssl_cert"`                               // For API < v2.0.1: SSL certificate contents (this is a not a path)
	Web_ui_https_key_path                  string         `json:"web_ui_https_key_path"`                  // For API ≥ v2.0.1: Path to SSL keyfile
	Web_ui_https_cert_path                 string         `json:"web_ui_https_cert_path"`                 // For API ≥ v2.0.1: Path to SSL certificate
	Dyndns_enabled                         bool           `json:"dyndns_enabled"`                         // True if server DNS should be updated dynamically
	Dyndns_service                         int64          `json:"dyndns_service"`                         // See list of possible values here below
	Dyndns_username                        string         `json:"dyndns_username"`                        // Username for DDNS service
	Dyndns_password                        string         `json:"dyndns_password"`                        // Password for DDNS service
	Dyndns_domain                          string         `json:"dyndns_domain"`                          // Your DDNS domain name
	Rss_refresh_interval                   int64          `json:"rss_refresh_interval"`                   // RSS refresh interval
	Rss_max_articles_per_feed              int64          `json:"rss_max_articles_per_feed"`              // Max stored articles per RSS feed
	Rss_processing_enabled                 bool           `json:"rss_processing_enabled"`                 // Enable processing of RSS feeds
	Rss_auto_downloading_enabled           bool           `json:"rss_auto_downloading_enabled"`           // Enable auto-downloading of torrents from the RSS feeds
	Rss_download_repack_proper_episodes    bool           `json:"rss_download_repack_proper_episodes"`    // For API ≥ v2.5.1: Enable downloading of repack/proper Episodes
	Rss_smart_episode_filters              string         `json:"rss_smart_episode_filters"`              // For API ≥ v2.5.1: List of RSS Smart Episode Filters
	Add_trackers_enabled                   bool           `json:"add_trackers_enabled"`                   // Enable automatic adding of trackers to new torrents
	Add_trackers                           string         `json:"add_trackers"`                           // List of trackers to add to new torrent
	Web_ui_use_custom_http_headers_enabled bool           `json:"web_ui_use_custom_http_headers_enabled"` // For API ≥ v2.5.1: Enable custom http headers
	Web_ui_custom_http_headers             string         `json:"web_ui_custom_http_headers"`             // For API ≥ v2.5.1: List of custom http headers
	Max_seeding_time_enabled               bool           `json:"max_seeding_time_enabled"`               // True enables max seeding time
	Max_seeding_time                       int64          `json:"max_seeding_time"`                       // Number of minutes to seed a torrent
	Announce_ip                            string         `json:"announce_ip"`                            // TODO
	Announce_to_all_tiers                  bool           `json:"announce_to_all_tiers"`                  // True always announce to all tiers
	Announce_to_all_trackers               bool           `json:"announce_to_all_trackers"`               // True always announce to all trackers in a tier
	Async_io_threads                       int64          `json:"async_io_threads"`                       // Number of asynchronous I/O threads
	Banned_IPs                             string         `json:"banned_IPs"`                             // List of banned IPs
	Checking_memory_use                    int64          `json:"checking_memory_use"`                    // Outstanding memory when checking torrents in MiB
	Current_interface_address              string         `json:"current_interface_address"`              // IP Address to bind to. Empty String means All addresses
	Current_network_interface              string         `json:"current_network_interface"`              // Network Interface used
	Disk_cache                             int64          `json:"disk_cache"`                             // Disk cache used in MiB
	Disk_cache_ttl                         int64          `json:"disk_cache_ttl"`                         // Disk cache expiry interval in seconds
	Embedded_tracker_port                  int64          `json:"embedded_tracker_port"`                  // Port used for embedded tracker
	Enable_coalesce_read_write             bool           `json:"enable_coalesce_read_write"`             // True enables coalesce reads & writes
	Enable_embedded_tracker                bool           `json:"enable_embedded_tracker"`                // True enables embedded tracker
	Enable_multi_connections_from_same_ip  bool           `json:"enable_multi_connections_from_same_ip"`  // True allows multiple connections from the same IP address
	Enable_os_cache                        bool           `json:"enable_os_cache"`                        // True enables os cache
	Enable_upload_suggestions              bool           `json:"enable_upload_suggestions"`              // True enables sending of upload piece suggestions
	File_pool_size                         int64          `json:"file_pool_size"`                         // File pool size
	Outgoing_ports_max                     int64          `json:"outgoing_ports_max"`                     // Maximal outgoing port (0: Disabled)
	Outgoing_ports_min                     int64          `json:"outgoing_ports_min"`                     // Minimal outgoing port (0: Disabled)
	Recheck_completed_torrents             bool           `json:"recheck_completed_torrents"`             // True rechecks torrents on completion
	Resolve_peer_countries                 bool           `json:"resolve_peer_countries"`                 // True resolves peer countries
	Save_resume_data_interval              int64          `json:"save_resume_data_interval"`              // Save resume data interval in min
	Send_buffer_low_watermark              int64          `json:"send_buffer_low_watermark"`              // Send buffer low watermark in KiB
	Send_buffer_watermark                  int64          `json:"send_buffer_watermark"`                  // Send buffer watermark in KiB
	Send_buffer_watermark_factor           int64          `json:"send_buffer_watermark_factor"`           // Send buffer watermark factor in percent
	Socket_backlog_size                    int64          `json:"socket_backlog_size"`                    // Socket backlog size
	Upload_choking_algorithm               int64          `json:"upload_choking_algorithm"`               // Upload choking algorithm used (see list of possible values below)
	Upload_slots_behavior                  int64          `json:"upload_slots_behavior"`                  // Upload slots behavior used (see list of possible values below)
	Upnp_lease_duration                    int64          `json:"upnp_lease_duration"`                    // UPnP lease duration (0: Permanent lease)
	Utp_tcp_mixed_mode                     int64          `json:"utp_tcp_mixed_mode"`                     // μTP-TCP mixed mode algorithm (see list of possible values below)
}

func (qt *apiTorrentInfo) CanResume() bool {
	return qt.State == "pausedUP" || qt.State == "stoppedUP" || qt.State == "pausedDL" || qt.State == "stoppedDL" ||
		qt.State == "queuedUP" || qt.State == "queuedDL" || qt.State == "error"
}

func (qt *apiTorrentInfo) CanPause() bool {
	return !qt.CanResume()
}

func (qbtorrent *apiTorrentInfo) ToTorrentState() string {
	state := ""
	switch qbtorrent.State {
	case "stalledUP", "queuedUP", "forcedUP", "uploading":
		state = "seeding"
	case "metaDL", "allocating", "stalledDL", "queuedDL", "forcedDL", "downloading":
		state = "downloading"
	case "pausedUP", "stoppedUP":
		state = "completed"
	case "pausedDL", "stoppedDL":
		state = "paused"
	case "checkingUP", "checkingDL", "checkingResumeData":
		state = "checking"
	case "error", "missingFiles", "unknown":
		state = "error"
	default:
		state = "unknown"
	}
	return state
}

func (qbtorrent *apiTorrentInfo) ToTorrent() *client.Torrent {
	torrent := &client.Torrent{
		InfoHash:           qbtorrent.Hash,
		Name:               qbtorrent.Name,
		TrackerDomain:      util.ParseUrlHostname(qbtorrent.Tracker),
		TrackerBaseDomain:  util.GetUrlDomain(qbtorrent.Tracker),
		Tracker:            qbtorrent.Tracker,
		State:              qbtorrent.ToTorrentState(),
		LowLevelState:      qbtorrent.State,
		Atime:              qbtorrent.Added_on,
		Ctime:              qbtorrent.Completion_on,
		ActivityTime:       qbtorrent.Last_activity,
		Downloaded:         qbtorrent.Downloaded,
		DownloadSpeed:      qbtorrent.Dlspeed,
		DownloadSpeedLimit: qbtorrent.Dl_limit,
		Uploaded:           qbtorrent.Uploaded,
		UploadSpeed:        qbtorrent.Upspeed,
		UploadedSpeedLimit: qbtorrent.Up_limit,
		Category:           qbtorrent.Category,
		SavePath:           qbtorrent.Save_path,
		ContentPath:        qbtorrent.ContentPath(),
		Tags:               util.SplitCsv(qbtorrent.Tags),
		Seeders:            qbtorrent.Num_complete,
		Size:               qbtorrent.Size,
		SizeCompleted:      qbtorrent.Completed,
		SizeTotal:          qbtorrent.Total_size,
		Leechers:           qbtorrent.Num_incomplete,
		Meta:               map[string]int64{},
	}
	torrent.Name, torrent.Meta = client.ParseMetaFromName(torrent.Name)
	return torrent
}

// Return the real content path (root folder) of torrent. It differs with qBittorrent Content_path in a special case:
// For a single file torrent. qb put abs path of that single file as Content_path, even if it has a wrap folder.
// E.g. if a torrent has a name "foo" and only one file "bar.txt" with a save path "/downloads",
// Content_path will be "/downloads/foo/bar.txt".
// This function returns "/downloads/foo" in this case.
func (qbtorrent *apiTorrentInfo) ContentPath() string {
	sep := qbtorrent.Sep()
	cp := qbtorrent.Content_path
	if !strings.HasPrefix(cp, qbtorrent.Save_path+sep) {
		return cp
	}
	relativepath := cp[len(qbtorrent.Save_path)+1:]
	i := strings.Index(relativepath, sep)
	if i == -1 {
		return cp
	}
	return qbtorrent.Save_path + sep + relativepath[:i]
}

// Return path sep (either '/' or '\') of this torrent.
func (qbtorrent *apiTorrentInfo) Sep() string {
	if !strings.Contains(qbtorrent.Save_path, `/`) && !strings.Contains(qbtorrent.Content_path, `/`) {
		return `\`
	}
	return `/`
}
