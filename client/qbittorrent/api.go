package qbittorrent

type apiTorrentContent struct {
	Index        int64   `json:"index"`        // File index
	Name         string  `json:"name"`         // File name (including relative path)
	Size         int64   `json:"size"`         // File size (bytes)
	Progress     float64 `json:"progress"`     // File progress (percentage/100)
	Priority     int64   `json:"priority"`     // File priority. See possible values here below
	Is_seed      bool    `json:"is_seed"`      // True if file is seeding/complete
	Piece_range  []int64 `json:"piece_range"`  // array	The first number is the starting piece index and the second number is the ending piece index (inclusive)
	Availability float64 `json:"availability"` // Percentage of file pieces currently available (percentage/100)
}

type apiCategoryStruct struct {
	Name     string `json:"name"`
	SavePath string `json:"savePath"`
}

type apiSyncMaindata struct {
	Server_state apiTransferInfo                `json:"server_state"`
	Tags         []string                       `json:"tags"`
	Categories   map[string](apiCategoryStruct) `json:"categories"`
	Torrents     map[string](apiTorrentInfo)    `json:"torrents"`
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
	Content_path       string  `json:"content_path"`       //	string	Absolute path of torrent content (root path for multifile torrents absolute file path for singlefile torrents)
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

func (qt *apiTorrentInfo) CanResume() bool {
	return qt.State == "pausedUP" || qt.State == "pausedDL" || qt.State == "queuedUP" || qt.State == "queuedDL" || qt.State == "error"
}

func (qt *apiTorrentInfo) CanPause() bool {
	return !qt.CanResume()
}
