module github.com/sagan/ptool

go 1.22.0

// Change Client.Connect signature to func (c *Client) Connect(requestHeader http.Header)
replace github.com/googollee/go-socket.io => ./go-socket.io

// workaround for some problem
replace github.com/hekmon/transmissionrpc/v2 => ./transmissionrpc

// workaround for https://github.com/c-bata/go-prompt/issues/228, with elyscape's fix applied
replace github.com/c-bata/go-prompt => ./go-prompt

// workaround for some problem
replace github.com/stromland/cobra-prompt => ./cobra-prompt

require (
	github.com/Noooste/azuretls-client v1.4.9
	github.com/PuerkitoBio/goquery v1.9.1
	github.com/anacrolix/torrent v1.55.0
	github.com/c-bata/go-prompt v0.2.6
	github.com/ettle/strcase v0.2.0
	github.com/glebarez/sqlite v1.11.0
	github.com/gofrs/flock v0.8.1
	github.com/google/shlex v0.0.0-20191202100458-e7afc7fbc510
	github.com/googollee/go-socket.io v1.8.0-rc.1.0.20230904084053-b044011d047b
	github.com/hekmon/transmissionrpc/v2 v2.0.1
	github.com/jpillora/go-tld v1.2.1
	github.com/mattn/go-runewidth v0.0.15
	github.com/pelletier/go-toml/v2 v2.2.1
	github.com/sirupsen/logrus v1.9.3
	github.com/spf13/cobra v1.8.0
	github.com/spf13/viper v1.18.2
	github.com/stromland/cobra-prompt v0.5.0
	golang.org/x/exp v0.0.0-20240416160154-fe59bbe5cc7f
	golang.org/x/net v0.24.0
	gorm.io/gorm v1.25.9
)

require (
	github.com/bradfitz/iter v0.0.0-20191230175014-e8f45d346db8 // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/fatih/color v1.16.0 // indirect
	github.com/gofrs/uuid v4.4.0+incompatible // indirect
	github.com/gomodule/redigo v1.9.2 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/gorilla/websocket v1.5.1 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/ncruces/go-strftime v0.1.9 // indirect
	github.com/refraction-networking/utls v1.6.4 // indirect
	github.com/remyoudompheng/bigfft v0.0.0-20230129092748-24d4a6f8daec // indirect
	modernc.org/libc v1.49.3 // indirect
	modernc.org/mathutil v1.6.0 // indirect
	modernc.org/memory v1.8.0 // indirect
	modernc.org/sqlite v1.29.7 // indirect
)

require (
	github.com/Noooste/fhttp v1.0.11 // indirect
	github.com/Noooste/utls v1.2.7 // indirect
	github.com/Noooste/websocket v1.0.3 // indirect
	github.com/anacrolix/missinggo v1.3.0 // indirect
	github.com/anacrolix/missinggo/v2 v2.7.3 // indirect
	github.com/andybalholm/brotli v1.1.0 // indirect
	github.com/andybalholm/cascadia v1.3.2 // indirect
	github.com/cloudflare/circl v1.3.7 // indirect
	github.com/fsnotify/fsnotify v1.7.0 // indirect
	github.com/glebarez/go-sqlite v1.22.0 // indirect
	github.com/hashicorp/go-cleanhttp v0.5.2 // indirect
	github.com/hashicorp/hcl v1.0.0 // indirect
	github.com/hekmon/cunits/v2 v2.1.0 // indirect
	github.com/huandu/xstrings v1.4.0 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/jinzhu/inflection v1.0.0 // indirect
	github.com/jinzhu/now v1.1.5 // indirect
	github.com/klauspost/compress v1.17.8 // indirect
	github.com/magiconair/properties v1.8.7 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-tty v0.0.5 // indirect
	github.com/mitchellh/mapstructure v1.5.0 // indirect
	github.com/pkg/term v1.2.0-beta.2 // indirect
	github.com/quic-go/quic-go v0.42.0 // indirect
	github.com/rivo/uniseg v0.4.7 // indirect
	github.com/sagikazarmark/locafero v0.4.0 // indirect
	github.com/sagikazarmark/slog-shim v0.1.0 // indirect
	github.com/sourcegraph/conc v0.3.0 // indirect
	github.com/spf13/afero v1.11.0 // indirect
	github.com/spf13/cast v1.6.0 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	github.com/subosito/gotenv v1.6.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/crypto v0.22.0 // indirect
	golang.org/x/sys v0.19.0 // indirect
	golang.org/x/term v0.19.0
	golang.org/x/text v0.14.0 // indirect
	gopkg.in/ini.v1 v1.67.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
