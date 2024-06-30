# ptool

自用的 PT ([Private trackers][]) 网站和 [BitTorrent][] 客户端辅助工具([Github](https://github.com/sagan/ptool))。提供全自动刷流(brush)、自动辅种(使用 IYUU 或 Reseed 等接口)、BT 客户端控制等功能。

主要特性：

- 使用 Go 开发的纯 CLI 程序。单文件可执行程序，没有外部依赖。支持 Windows / Linux、x64 / arm64 等多种环境、架构。
- 无状态(stateless)：程序自身不保存任何状态、不在后台持续运行。“刷流”等任务需要使用 cron job 等方式定时运行本程序。
- 使用简单。只需 5 分钟时间，配置 BitTorrent 客户端地址、PT 网站地址和 cookie 即可开始全自动刷流。
- 目前支持的 BitTorrent 客户端： qBittorrent v4.1+ / Transmission (<= v3.0)。
  - 推荐使用 qBittorrent。Transmission 客户端未充分测试。
- 目前支持的 PT 站点：绝大部分使用 nexusphp 的网站；M-Team(馒头)。
  - 测试过支持的站点：U2、冬樱、红叶、聆音、铂金家、若干不可说的站点等。
  - 未列出的大部分 np 站点应该也支持。除了个别魔改 np 很厉害的站点可能有问题。
  - 支持通过 [CookieCloud][] 自动同步站点 cookie 或导入站点。
- 刷流功能(brush)：
  - 不依赖 RSS。直接抓取站点页面上最新的种子。
  - 无需配置选种规则。自动跳过非免费的和有 HR 的种子；自动筛选适合刷流的种子。
  - 无需配置删种规则。自动删除已无刷流价值的种子；自动删除免费时间到期并且尚未下载完成的种子；硬盘空间不足时也会自动删种。
- 其它 PT 站点功能：搜索种子、批量下载种子、发布种子。
- BitTorrent 客户端控制功能：提供完整的管理、控制 BitTorrent 客户端的功能。
- Torrent 文件 / BitTorrent 协议相关的各种辅助功能。部分命令为[松鼠党][]特别优化，支持与 [rclone][] 整合（例如：`verifytorrent`命令能够直接检测 .torrent 种子的内容文件在 rclone 云存储上是否存在）。
- 自动模仿浏览器访问 PT 站点，能够绕过大多数站点的 CF 盾 (impersonate 特性)。

## 下载

- [开发版本](https://ci.appveyor.com/project/sagan/ptool/build/artifacts) (根据 master 分支最新代码自动构建)
- [稳定版本](https://github.com/sagan/ptool/releases)

## 快速开始（刷流）

将本程序的可执行文件 ptool (Linux) 或 ptool.exe (Windows) 放到任意目录（推荐放到 PATH 路径里），运行 `ptool config create` 创建本程序使用的 ptool.toml 配置文件。创建的文件位于当前系统用户主目录的 `.config/ptool/` 路径下。编辑这个文件配置 BT 客户端和 PT 站点信息：

```toml
[[clients]]
name = "local"
type = "qbittorrent" # 客户端类型。目前主要支持 qBittorrent v4.1+。需要启用 Web UI
url = "http://localhost:8080/" # qBittorrent web UI 地址
username = "admin" # QB Web UI 用户名
password = "adminadmin" # QB Web UI 密码

[[sites]]
type = "keepfrds"
cookie = "cookie_here" # 浏览器 F12 获取的网站 cookie
```

然后运行 `ptool brush local keepfrds` 即可执行刷流任务。程序会从 keepfrds 站点获取最新的种子、根据一定规则筛选出适合的种子添加到 local 这个 BT 客户端里，同时自动从 BT 客户端里删除（已经没有上传的）旧的刷流种子。刷流任务添加到客户端里的种子会放到 `_brush` 分类(Category)里。程序只会对这个分类里的种子进行管理或删除等操作。

使用 Linux cron job / Windows 计划任务 (taskschd.msc) 等方式定时执行上面的刷流任务命令（例如每隔 10 分钟执行一次）即可。

## 配置文件

程序支持使用 toml 或 yaml 格式的配置文件（`ptool.toml` 或 `ptool.yaml`），推荐使用前者。

将 ptool.toml 配置文件放到当前操作系统用户主目录下的 ".config/ptool/" 路径下（推荐）:

- Linux: `~/.config/ptool/ptool.toml`
- Windows: `%USERPROFILE%\.config\ptool\ptool.toml`

如果临时测试，也可以将 ptool.toml 配置文件直接放到程序启动时的当前目录(cwd)下。

配置文件里可以使用 `[[clients]]` 和 `[[sites]]` 区块添加任意多个 BT 客户端和站点。

`[[sites]]` 区块有两种配置方式：

```toml
# 方式 1（推荐）：直接使用站点 ID 或 alias 作为类型(type)。无需手动输入站点 url。
[[sites]]
#name = "keepfrds" # (可选)手动指定站点名称。如果不指定，默认使用其 type 作为 name
type = "keepfrds"
cookie = "cookie_here" # 浏览器 F12 获取的网站 cookie

# 方式 2：使用通用的 nexusphp 等站点架构类型，需要手动指定站点名称(name)、站点 url 和其他参数。
[[sites]]
name = "keepfrds"
type = "nexusphp" # 通用站点架构类型。可选值: nexusphp|gazellepw|unit3d|tnode|discuz|mtorrent
url = "https://pt.keepfrds.com/" # 站点首页 URL
cookie = "cookie_here" # 浏览器 F12 获取的网站 cookie
```

推荐使用“方式 1”。程序内置了对大部分国内 NexusPHP PT 站点的支持。站点 type 通常为 PT 网站域名的主体部分（不含次级域名和 TLD 部分），例如 BTSCHOOL ( https://pt.btschool.club/ )的站点 type 是 btschool。部分 PT 网站也可以使用别名(alias)配置，例如 M-TEAM ( https://kp.m-team.cc/ )在本程序配置文件里的 type 设为 "m-team" 或 "mteam" 均可。运行 `ptool sites` 查看所有本程序内置支持的 PT 站点列表。本程序没有内置支持的 PT 站点必须通过“方式 2”配置。 （注：部分非 NP 架构站点本程序目前只支持自动辅种、查看站点状态，暂不支持刷流、搜索站点种子等功能）

注：新版 M-Team（馒头）不使用 Cookie 鉴权；其配置方式参考`ptool.example.toml` 示例配置文件里说明。

配置好站点后，使用 `ptool status <site> -t` 测试（`<site>`参数为站点的 name）。如果配置正确且 Cookie 有效，会显示站点当前登录用户的状态信息和网站最新种子列表。

程序支持自动与浏览器同步站点 Cookies 或导入站点信息。详细信息请参考本文档 "cookiecloud" 命令说明部分。

参考程序代码 config/ 目录下的 `ptool.example.toml` 示例配置文件了解常用配置项信息。

查看程序代码 [config/config.go](https://github.com/sagan/ptool/blob/master/config/config.go) 文件里的 type ConfigStruct struct 获取全部可配置项信息。

## 程序功能

所有功能通过启动程序时传入的第一个”命令“参数区分：

```
ptool <command> args... [flags]
```

所有可用的 `<command>` 包括:

- brush : 自动刷流。
- iyuu : 使用 [IYUU][] 接口自动辅种。
- reseed : 使用 [Reseed][] 接口自动辅种。
- batchdl : 批量下载站点的种子。
- status : 显示 BT 客户端或 PT 站点当前状态信息。
- stats : 显示刷流任务流量统计。
- search : 在某个站点搜索指定关键词的种子。
- dynamicseeding : 全站动态保种。
- add : 将种子添加到 BT 客户端。
- dltorrent : 下载站点的种子(.torrent 文件)。
- publish : 发布(上传)种子到站点。
- BT 客户端控制命令集: clientctl / show / pause / resume / delete / reannounce / recheck / getcategories / createcategory / deletecategories / setcategory / gettags / createtags / deletetags / addtags / removetags / renametag / edittracker / addtrackers / removetrackers / setsavepath / setsharelimits / checktag / export 。
- parsetorrent : 显示种子(.torrent)文件信息。
- verifytorrent : 测试种子(.torrent)文件与硬盘上的文件内容一致。
- maketorrent : 制作种子(.torrent)文件。
- edittorrent : 编辑（修改）种子(.torrent)文件内容。
- partialdownload : 拆包下载。
- xseedadd : 手动添加辅种种子到客户端。
- findalone : 查找下载目录里的未做种文件。
- markinvalidtracker : 标记 BT 客户端里 Tracker 状态异常的种子。
- movesavepath : 修改本地 BT 客户端里的种子内容文件保存路径。
- cookiecloud : 使用 [CookieCloud][] 同步站点的 Cookies 或导入站点。
- sites : 显示本程序内置支持的所有 PT 站点列表。
- config : 显示当前 ptool.toml 配置文件信息。
- shell : 进入交互式终端环境。
- version : 显示本程序版本信息。

运行 `ptool` 查看程序支持的所有命令列表；运行 `ptool <command> -h` 查看指定命令的参数格式和使用说明。本程序目前仍位于 0.x.x 版本的开发阶段，各个命令、命令参数名称或格式、配置文件配置项等可能会经常变动。

全局参数(flags)：

- --config string : 手动指定使用的 ptool.toml 配置文件路径。
- -v, -vv, -vvv : verbose。输出更多的日志信息（v 出现的次数越多，输出的日志越详细）。

### 刷流 (brush)

```
ptool brush <client> <site>... [flags]
```

刷流任务从指定的站点获取最新种子，选择适当的种子加入 BT 客户端；并自动从客户端中删除旧的（已没有上传速度的）刷流任务种子及其文件。刷流任务的目标是使 BT 客户端的上传速度达到软件中设置的上传速度上限（如果客户端里没有设置上传速度上限，本程序默认使用 10MiB/s 这个值），如果当前 BT 客户端的上传速度已经达到或接近了上限（不管上传是否来源于刷流任务添加的种子），程序不会添加任何新种子。

参数

- `<client>` : 配置文件里定义的 BT 客户端 name。
- `<site>` : 配置文件里定义的 PT 站点 name。

可以提供多个 `<site>` 参数。程序会按随机顺序从提供的 `<site>` 列表里的各站点获取最新种子、筛选一定数量的合适的种子添加到 BT 客户端。可以将同一个站点名重复出现多次以增加其权重，使刷流任务添加该站点种子的几率更大。如果提供的所有站点里都没有找到合适的刷流种子，程序也不会添加种子到客户端。

示例

```
# 使用 local 这个 BT 客户端，刷流 mteam 站点
ptool brush local mteam
```

选种（选择新种子添加到 BT 客户端）规则：

- 不会选择有以下任意特征的种子：不免费、存在 HnR 考查、免费时间临近截止。
- 部分站点存在“付费”种子（下载或汇报时会扣除积分），这类种子也不会被选择。
- 发布时间过久的种子也不会被选择。
- 种子的当前做种、下载人数，种子大小等因素也都会考虑。

删种（删除 BT 客户端里旧的刷流种子）规则：

- 未下载完成的种子免费时间临近截止时，删除种子或停止下载（只上传模式）。
- 硬盘剩余可用空间不足（默认保留 5GiB）时，开始删除没有上传速度的种子。
- 未下载完成的种子，如果长时间没有上传速度或上传/下载速度比例过低，也可能被删除。

刷流任务添加到客户端里的种子会放到 `_brush` 分类(category)里。程序只会对这个分类里的种子进行管理或删除等操作。不会干扰 BT 客户端里其它正常的下载任务。如果需要永久保留某个刷流任务添加的种子（防止其被自动删除），在 BT 客户端里更改其分类即可。

其它说明：

- No-Add 模式：如果 BT 客户端里当前存在 `_noadd` 这个标签(tag)，刷流任务不会添加任何新种子到客户端。

### 自动辅种 (iyuu)

iyuu 命令通过 [IYUU 接口][] 提供自动辅种(cross seed)功能。本功能直接访问 IYUU 的服务器，本机上不需要安装 / 运行 IYUU 客户端。

#### IYUU 配置

如果是第一次使用 "IYUU"，首先需要在 [IYUU 网站][] 上微信扫码申请 IYUU 令牌（token）。在 ptool.toml 配置文件里配置 IYUU token：

```
iyuuToken = "IYUU0011223344..."
```

然后使用 IYUU 支持的任意合作站点的 uid 和 passkey 激活和绑定(bind) IYUU token，命令格式如下：

```
ptool iyuu bind --site zhuque --uid 123456 --passkey 0123456789abcdef
```

所有参数均必须提供

- --site : 用于验证的 PT 站点名。可以使用 `ptool iyuu sites -b` 命令查询 IYUU 支持的合作站点列表。
- --uid : 对应 PT 站点的用户 uid（数字）。在 PT 网站的个人页面获取。
- --passkey : 对应 PT 站点的用户 passkey。在 PT 网站的个人页面获取。

其他说明：

- 使用 `ptool iyuu sites` 查看 IYUU 支持的所有可辅种站点列表。
- 使用 `ptool iyuu status` 查询当前 IYUU token 的激活和绑定状态。

#### 使用 IYUU 自动辅种

```
ptool iyuu xseed <client>...
```

可以提供多个 client。程序会获取这些 client 里正在做种的种子信息，通过 IYUU 接口查询可以辅种的种子并将其自动添加到对应客户端里。注意只有在本程序的 ptool.toml 配置文件里添加的站点的种子才会被辅种。

iyuu xseed 子命令支持很多可选参数。运行 `ptool iyuu xseed -h` 查看所有可选参数使用说明。

添加的辅种种子默认跳过客户端 hash 校验并立即开始做种。本程序会对客户端里目标种子和 IYUU 接口返回的候选辅种种子的文件列表进行比较（文件路径、大小），只有完全一致才会添加辅种种子。添加的辅种种子会打上 `_xseed` 标签。

## 自动辅种 (reseed)

reseed 命令使用 [Reseed][] 提供的接口自动辅种。

### reseed 配置

首先在 [Reseed 官网][] 注册（需要使用指定 PT 站点验证），然后在 ptool.toml 配置文件里配置 Reseed 的用户信息：

```
reseedUsername = 'username'
reseedPassword = 'password'
```

### 使用 Reseed 辅种

Reseed 的辅种原理是扫描本地硬盘的“下载文件夹” (QB 的 save path)生成其中所有内容的元信息索引（文件名 & 大小），然后上传索引到服务器并自动找到匹配内容元信息的站点种子：

reseed 命令设计有两种使用方式：

#### 方式 1

```
ptool reseed match --download "D:\Downloads"
ptool xseedadd local "C:\Users\<username>\.config\ptool\reseed\*.torrent"
```

首先扫描 D:\Downloads 里的文件，通过 Reseed 查询匹配的站点种子，将找到的所有 .torrent 种子文件下载到本地（下载文件默认保存路径：ptool.toml 配置文件所在目录的 "reseed" 子文件夹里）。注意只有在本程序的 ptool.toml 配置文件里添加的站点的种子才会被下载。

然后使用 ptool xseedadd 命令将所有下载的 Reseed 辅种种子添加到本地 BT 客户端。

这种方式下，BitTorrent 客户端与运行 ptool 程序环境的文件系统可以不一致（e.g. BitTorrent 客户端运行在 Docker 里，而 ptool 运行在宿主机里）。

#### 方式 2

```
ptool reseed match --download --use-comment-meta "D:\Downloads"
ptool verifytorrent --check --rename-fail --use-comment-meta "C:\Users\root\.config\ptool\reseed\*.torrent"
ptool add local --use-comment-meta --skip-check "C:\Users\<username>\.config\ptool\reseed\*.torrent"
```

这种方式仅能用于对运行在本机上（文件系统与运行 ptool 程序的环境相同）的 BT 客户端自动辅种。指定了 --use-comment-meta 参数，在下载种子时将其内容在硬盘上的保存路径(save path) 写入 .torrent 文件的 comment 字段。

（可选）然后使用 "ptool verifytorrent" 命令手动校验这些种子与硬盘上文件是否匹配，将校验失败的种子重命名为 ".torrent.fail"。

最后使用 "ptool add" 命令并同样指定 --use-comment-meta 参数，直接将种子添加到 BT 客户端并从种子 comment 字段读取并应用保存路径。这种方式添加的种子在客户端里无需存在已有的内容完全相同的种子。

### 其它功能

```
# 查看 Reseed 账号状态
ptool reseed status

# 查看 Reseed 支持的所有可辅种 PT 站点列表
ptool reseed sites
```

### BT 客户端控制命令集

提供了一系列管理、控制 BT 客户端的命令。

#### 读取/修改 BT 客户端配置 (clientctl)

```
ptool clientctl <client> [<option>[=value] ...]
```

clientctl 命令可以显示或修改指定 name 的 BT 客户端的配置参数。

支持的参数(`<option>`) 列表：

- global_download_speed_limit : 全局下载速度上限。
- global_upload_speed_limit : 全局上传速度上限。
- global_download_speed : (只读)当前下载速度。
- global_upload_speed : (只读)当前上传速度。
- free_disk_space : (只读)默认下载目录的剩余磁盘空间(-1: Unknown)。
- save_path : 默认下载目录。
- `qb_*` : qBittorrent 的所有 [application Preferences](<https://github.com/qbittorrent/qBittorrent/wiki/WebUI-API-(qBittorrent-4.1)#get-application-preferences>) 配置项，例如 "qb_start_paused_enabled"。
- `tr_*` : transmission 的所有 [Session Arguments](https://github.com/transmission/transmission/blob/3.00/extras/rpc-spec.txt#L482) 配置项(转换为 snake_case 格式)，例如 "tr_config_dir"。

示例：

```
# 获取 local 客户端所有参数当前值
ptool clientctl local

# 设置 local 客户端的全局上传速度限制为 10MiB/s
ptool clientctl local global_upload_speed_limit=10M
```

#### 显示信息 / 暂停 / 恢复 / 删除 / 强制汇报 / 强制检测 Hash 客户端里种子 (show / pause / resume / delete / reannounce / recheck)

命令格式均为：

```
ptool <command> <client> [flags] [<infoHash>...]
```

`<infoHash>` 参数为指定的 BT 客户端里需要操作的种子的 infoHash 列表。也可以使用以下特殊值参数操作多个种子：

- `_all` : 所有种子
- `_done` : 所有已下载完成的种子（无论是否正在做种）
- `_undone` : 所有未下载完成的种子
- `_active` : 当前正在活动（上传或下载）的种子
- `_error` : 状态为“出错”的种子
- `_downloading` / `_seeding` / `_paused` / `_completed` : 状态为正在下载 / 做种 / 暂停下载 / 下载完成(但未做种)的种子

也可以使用以下条件 flags 筛选种子：

- `--category string` : 指定分类的种子
- `--tag string` : 含有指定标签的种子（可以用逗号分隔多个标签，种子含有其中任意标签均视为符合条件）
- `--filter string` : 种子名称中包含指定文字的种子

示例：

```
# 强制立即汇报所有种子
ptool reannounce local _all

# 恢复下载/做种所有种子
ptool resume local _all

# 暂停 abc 分类下的所有正在下载种子
ptool pause local --category abc _downloading

# 从客户端删除指定种子（默认同时删除文件）。默认会提示确认删除，除非指定 --force 参数
ptool delete local 31a615d5984cb63c6f999f72bb3961dce49c194a

# 特别的，如果 show 命令只提供一个 infoHash 参数，会显示该种子的所有详细信息
ptool show local 31a615d5984cb63c6f999f72bb3961dce49c194a
```

除 `show` 以外的命令可以只传入一个特殊的 `-` 作为参数，视为从 stdin 读取 infoHash 列表。而 `show` 命令提供很多参数可以用于筛选种子，并且可以使用 `--show-info-hash-only` 参数只输出匹配的种子的 infoHash。因此可以组合使用 `show` 命令和其它命令，例如：

```
# 删除 local 客户端里 "rss" 分类里已经下载完成超过 5 天的种子
ptool show local --category rss --completed-before 5d --show-info-hash-only | ptool delete local --force -
```

#### 管理 BT 客户端里的的种子分类 / 标签 / Trackers 等(getcategories / createcategory / deletecategories / setcategory / gettags / createtags / deletetags / addtags / removetags / renametag / edittracker / addtrackers / removetrackers / setsavepath / setsharelimits / checktag)

```
# 获取所有分类
ptool getcategories <client>

# 新增分类(也可用于修改已有分类的下载目录)
ptool createcategory <client> <category> --save-path "/root/downloads"

# 删除分类
ptool deletecategories <client> <category>...

# 修改种子的所属分类
ptool setcategory <client> <category> <infoHashes>...

# 获取所有标签(tag)
ptool gettags <client>

# 创建新的标签
ptool createtags <client> <tags>...

# 删除标签
ptool deletetags <client> <tags>...

# 为客户端里种子添加 tag
ptool addtags <client> <tags> <infoHashes>...

# 为客户端里种子删除 tag
ptool removetags <client> <tags> <infoHashes>...

# 重命名客户端里的 tag
ptool renametag <client> <old-tag> <new-tag>

# 修改种子的 tracker。只有 old tracker 存在的种子会被修改
ptool edittracker <client> _all --old-tracker "https://..." --new-tracker "https://..."

# 只替换种子 tracker 的 host (域名)部分
ptool edittracker <client> _all --old-tracker old-tracker.com --new-tracker new-tracker.com --replace-host

# 将所有 host 相匹配的旧 Tracker 替换为提高的新的 Tracker 地址
ptool edittracker <client> _all --old-tracker tracker.hdtime.org --new-tracker "https://tracker.hdtime.org/announce.php?passkey=123456" --replace-host

# 为种子增加 tracker
ptool addtrackers <client> <infoHashes...> --tracker "https://..."

# 删除种子的 tracker
ptool removetrackers <client> <infoHashes...> --tracker "https://..."

# 修改种子内容的保存路径
ptool setsavepath <client> <savePath> [<infoHash>...]

# (qbittorrent only) 设置种子最大分享比例(Up/Dl)、最长做种时间(秒)等。
# 使用 "ptool add" 命令添加种子时也可以设置同样参数。
ptool setsharelimits <client> [<infoHash>...] --ratio-limit 2 --seeding-time-limit 86400

# 检测客户端里是否存在某个 tag。If exists, exit with 0。
ptool checktag <client> <tag>
```

#### 导出客户端种子 (export)

```
ptool export <client> <infoHash>...
```

导出客户端里的种子为 .torrent 文件。

该功能支持一个特殊的 `--use-comment-meta` 参数，会将客户端里种子的分类(category)、标签(tags)、保存路径(savePath)等元信息保存到导出的 .torrent 文件的 "comment" 字段里。`ptool add` 命令使用同样参数可以在添加种子时使用 .torrent 文件 "comment" 字段里的元信息。该功能的设计目的是在重装 qBittorrent 或重装操作系统后恢复种子，也可以用于转移种子做种客户端。

### 显示 BT 客户端或 PT 站点状态 (status)

```
ptool status <clientOrSite>...
```

显示指定 name 的 BT 客户端或 PT 站点的当前状态信息。可以提供多个名称。

显示的信息包括：

- BT 客户端：显示当前下载 / 上传速度和其上限，硬盘剩余可用空间。
- PT 站点：显示用户名、上传量、下载量。

可选参数：

- -t : 显示 BT 客户端或站点的种子列表（BT 客户端：当前活动的种子；PT 站点：最新种子）。
- -f : 显示完整的种子列表信息。

### 显示刷流任务流量统计 (stats)

```
ptool stats [client...]
```

显示 BT 客户端的刷流任务流量统计信息（下载流量、上传流量总和）。本功能默认不启用，如需启用，在 ptool.toml 配置文件的最上方里增加一行：`brushEnableStats = true` 配置项。启用刷流统计后，刷流任务会使用 ptool.toml 配置文件相同目录下的 "ptool_stats.txt" 文件存储所需保存的信息。

只有刷流任务添加和管理的 BT 客户端的种子（即 `_brush` 分类的种子）的流量信息会被记录和统计。目前设计只有在刷流任务从 BT 客户端删除某个种子时才会记录和统计该种子产生的流量信息。

### 添加种子到 BT 客户端 (add)

```
ptool add <client> <torrentFileNameOrIdOrUrl>...
```

参数可以是本地硬盘里的种子文件名(支持 `*` 通配符)、站点的种子 id 或 url。例如：

```
ptool add local *.torrent
```

以上命令将当前目录下所有 ".torrent" 种子文件添加到 "local" BT 客户端。

```
ptool add local mteam.488424
ptool add local --site mteam 488424
ptool add local "https://kp.m-team.cc/details.php?id=488424"
ptool add local "https://kp.m-team.cc/download.php?id=488424"
```

以上几条命令均可以将 M-Team 站点上 ID 为 [488424](https://kp.m-team.cc/details.php?id=488424&hit=1) 的种子添加到 "local" BT 客户端。

参数也支持传入公开 BT 网站的种子下载链接或 `magnet:` 磁力链接地址。

特别的，如果参数只有 1 个 "-"，视为从 stdin 读取种子列表；也支持直接从 stdin 传入 .torrent 文件内容。

### 下载站点的种子

```
ptool dltorrent <torrentIdOrUrl>...
```

参数与 "add" 命令相似（不支持本地文件名）。将站点的种子文件下载到本地。

可选参数：

- --download-dir : 下载的种子文件保存路径。默认为当前目录(.)。

### 搜索 PT 站点种子 (search)

```
ptool search <sites> <keyword>
```

`<sites>` 参数为需要所搜索的 PT 站点，可以使用 "," 分割提供多个站点。可以使用 `_all` 搜索所有已配置的 PT 站点。

使用 `ptool add` 命令将搜索结果列表中的种子添加到 BT 客户端。

### 批量下载种子 (batchdl)

提供一个 batchdl 命令用于批量下载 PT 网站的种子（别名：ebookgod）。默认按种子体积大小升序排序、跳过死种和已经下载过的种子。

基本用法：

```
# 默认显示找到的种子列表
ptool batchdl <site>

# 下载找到的种子到当前目录
ptool batchdl <site> --download

# 直接将种子添加到 "local" BT 客户端里
ptool batchdl <site> --add-client local
```

此命令提供非常多的配置参数。部分参数：

- --max-torrents int : 最多下载多少个种子。默认 -1 (无限制，一直运行除非手动 Ctrl + C 停止)。
- --sort string : 站点种子排序方式：size|time|name|seeders|leechers|snatched|none (default size)
- --order string : 排序顺序：asc|desc。默认 asc。
- --min-torrent-size string : 种子大小的最小值限制 (e.g. "100MiB", "1GiB")。默认为 "-1"（无限制）。
- --max-torrent-size string : 种子大小的最大值限制。默认为 "-1"（无限制）。
- --max-total-size string : 下载种子内容总体积最大值限制 (e.g. "512GiB", "1TiB")。默认为 "-1"（无限制）。
- --free : 只下载免费种子。
- --no-hr : 跳过存在 HR 的种子。
- --no-paid : 跳过"付费"的种子。(部分站点存在"付费"种子，第一次下载或汇报时扣除积分)
- --base-url : 手动指定种子列表页 URL，例如："special.php"、"torrents.php?cat=100"。
- --start-page string : 指定起始页面序号。
- --one-page : 只抓取 1 页种子。
- --add-category-auto : 添加种子到 BT 客户端时，将其分类(Category)设为站点名。

实际使用场景示例：

```
ptool batchdl kamept --tag "外语音声,同人志" --sort none --start-page 0 --free --one-page --add-client local --add-category-auto
```

获取 kamept 首页最新的 "外语音声"或"同人志"分类里的免费种子并添加到 local 客户端。使用 crontab 定时运行即可，可以实现比 RSS 更细致的筛选，并且不依赖站点。

### 全站动态保种 (dynamicseeding) (试验性功能)

```
ptool dynamicseeding {client} {site}
```

dynamicseeding 命令自动从指定站点下载亟需保种的种子并做种。

首先在 ptoo.toml 配置文件里设置对于指定站点允许使用多少硬盘空间进行动态保种：

```
[[sites]]
type = 'kamept'
cookie = '...'
dynamicSeedingSize = '500GiB' # 动态保种使用硬盘空间
dynamicSeedingTorrentMaxSize = '20GiB' # 动态保种单个种子大小上限
```

然后定期运行以下命令即可，参数分别为动态保种使用的 BT 客户端、站点名：

```
ptool dynamicseeding local kamept
```

“动态保种”功能详细说明：

- 如果“可用空间”（可用空间为 dynamicSeedingSize 减去 BT 客户端里当前该站点所有动态保种种子大小之和）有空余，程序会从站点下载当前亟需保种（有断种风险）的种子并做种。
- 默认仅会下载免费并且没有 HR 的种子。
- 动态保种的种子会放到 qBittorrent 的 `dynamic-seeding-<sitename>` 分类里，并且打上 `site:<sitename>` 标签。
- 如果“可用空间”不足并且有新的亟需保种的种子，程序会删除 BT 客户端里该站点的动态保种种子里已经不再有断种风险的种子，以腾出空间下载新的种子。对于站点已经删除的种子，程序也会从 BT 客户端里删除。
- 对于 BT 客户端里正在做种的动态保种种子，如果其当前做种人数 < 4，程序在任何情况下都不会自动删除该种子（即使“可用空间”不足）。
- 程序也不会自动删除含有 `nodel` 标签的动态保种种子。
- 用户自行下载的种子，也可以将其放到 `dynamic-seeding-<sitename>` 分类并打上 `site:<sitename>` 标签，以允许动态保种功能对其进行管理并在需要时删除其以腾出空间下载新的种子（注意分类和标签两者都必须设置）。

### 发布(上传)种子 (publish)

示例：

```
ptool publish --site kamept --client local --check-existing --save-path /downloads/i
```

publish 命令能够自动发布种子到 PT 站点。以上面示例为例，对于 --save-path 目录里的每一个内容文件夹，将执行以下步骤：

1. 检测文件夹里是否存在 `metadata.nfo` 文件。如果有，认为该文件夹是合法的种子内容文件夹(content-path)。
2. 读取 metadata.nfo 里的内容生成上传种子的元信息（标题、副标题、标签、番号等）。
3. 根据种子的元信息，检测站点里是否已经存在相同内容种子，如存在则停止发布种子。
4. 对该内容文件夹制种。生成的种子位于文件夹里的 `.torrent` 文件。
5. 如果内容文件夹里存在 `cover.*` (任意图片格式扩展名，例如 .webp / .png / .jpg)，将该图片作为种子的描述，上传到站点的图床里。
6. 自动通过站点上传种子接口发布种子，将站点生成的发布后种子下载到内容文件夹里的 `.<sitename>.torrent` 文件里。
7. 将下载的 `.<sitename>.torrent` 种子添加到 local 客户端并开始做种。

#### metadata.nfo 元文件

ptool 目前只支持读取 plain text 格式的 metadata.nfo 作为元信息文件，格式示例如下：

```
---
title: どの娘に耳かきしてもらう?
author: Butterfly Dream
narrator: 浅見ゆい
tags: ASMR, WAV, ほのぼの, バイノーラル/ダミヘ, ボイス・ASMR, ラブラブ/あまあま, 健全, 日常/生活, 癒し, 耳かき
number: RJ01205338
source: dlsite
---

作品コンセプト

合計6キャラクターの耳かきボイスを収録したオムニバス形式の作品です。
様々なシチュエーションで、思う存分耳かきを楽しむことができます。
※すべてのトラックで、耳かき、梵天、耳ふーの流れとなっております。
```

文件的开头是 Jekyll 的 [YAML Front Matter](https://jekyllrb.com/docs/front-matter/) 风格的元信息字段。其中的 "number" (番号) 字段被用于检测同样内容种子在站点上是否存在。

#### 支持的站点

ptool 会根据元文件信息生成站点发布种子接口需要提交的 Form 字段内容。字段内容使用 Jinja 模板渲染生成。默认 NP 站点的发布种子字段包括：

- `name`: 种子标题。默认为 `{% if number %}[{{number}}]{% endif %}{% if author %}[{{author}}]{% endif %}{{title}}`。示例：`[RJ01205338][Butterfly Dream]どの娘に耳かきしてもらう?`。
- `desc`: 种子描述。

大部分 NP 站点发布种子时除标题外还存在其它必填字段，例如 "type" (种子分类)。目前 ptool 仅支持对部分站点自动生成这些必填字段值；对于其它站点，需要在 ptool.toml 配置文件里手动指定生成这些字段值的模板，例如：

```
[uploadTorrentAdditionalPayload]
type = """
{% if "ボイス・ASMR" in tags %}420
{% elif "ゲーム" in tags %}415
{% else %}999
{% endif %}
"""
```

模板渲染结果开头和末尾的空白字段会被自动去除。

如果需要上传 cover.jpg 等种子描述图片，大部分站点还需要在 ptool.toml 里配置站点的图床接口，示例配置（注释掉的配置项为默认值）

```

imageUploadUrl = 'https://pic.example.com/'
#imageUploadFileField = 'file'
#imageUploadResponseUrlField = 'url'
#imageUploadPayload = '' # e.g. 'foo=1&bar=2'
```

默认配置下，将对站点的图床接口发起 `multipart/form-data` 类型的 POST 请求，将图片文件放到 form data 的 `file` 字段。然后从 response 的 json object 的 `url` 字段获取上传的图片的 URL。

目前程序仅内置支持 kamept 的图床和上传种子必填字段生成。

### 显示种子文件信息 (parsetorrent)

```
ptool parsetorrent <torrentFileNameOrIdOrUrl>...
```

显示种子文件的元信息。参数是本地硬盘里的种子文件名，或站点的种子 id 或 url（参考 "add" 命令说明）。

### 校验种子文件与硬盘内容是否一致 (verifytorrent)

```
ptool verifytorrent <torrentFileNameOrIdOrUrl>...
```

默认只对比文件元信息(文件名、文件大小)。如果指定 --check 或 --check-quick 参数，会对硬盘上文件内容进行 hash 校验。

必选参数（必须且只能提供以下几个参数中的其中 1 个参数）：

- `--save-path <path>` : 种子内容保存路径(下载文件夹)。可以用于校验多个 torrent 文件。
- `--content-path <path>` : 种子内容路径(root folder 或单文件种子的文件路径)。只能用于校验 1 个 torrent 文件。
- `--use-comment-meta` : 读取并使用种子 .torrent 文件的 comment 字段里存储的 save_path 信息。设计用于配合其它命令(例如 `ptool export`)使用。
- `--rclone-lsjson-file <file>` : 元信息索引文件名，其内容为 [rclone][] 的 `rclone lsjson --recursive <path>` 命令输出。rclone 的 `<path>` 被认为是种子内容的保存路径。参考 [rclone lsjson][] 命令的文档。
- `--rclone-save-path <path>` : 类似 `--rclone-lsjson-file`，但直接指定 `<path>` 路径。ptool 将运行 `rclone lsjson --recursive <path>` 并读取其输出。E.g. "remote:Downloads"。

`--rclone-lsjson-file` 和 `--rclone-save-path` 参数的设计目的是用于检测种子的内容文件在云存储上是否存在。

其它参数：

- `--check` : 对硬盘上文件进行完整 hash 校验。
- `--check-quick` : 对硬盘上文件进行快速 hash 校验，每个文件只对第 1 个和最后 1 个 piece 进行 hash 计算。

示例：

```
ptool verifytorrent file.torrent --save-path D:\Downloads --check

ptool verifytorrent MyTorrent.torrent --content-path D:\Downloads\MyTorrent --check

ptool verifytorrent *.torrent --rclone-save-path remote:Downloads
```

### 制作种子 (maketorrent)

maketorrent 命令根据提供的“内容文件(夹)”生成种子(.torrent)文件：

示例：

```
# 生成 MyVideos.torrent
ptool maketorrent ./MyVideos
```

常用参数：

- `--public` : 添加常见的公开 Tracker 服务器地址到生成的种子里。
- `--private` : 将生成的种子标记为非公开 (Private Tracker 标记）。
- `--tracker` : 手动添加 tracker 地址到生成的种子里。

“内容文件夹”里的一些临时或隐藏类型文件（例如 `.*`, `*.tmp`, `Thumbs.db` 等）默认会被自动忽略，不会被添加到种子里。

### 编辑种子文件 (edittorrent)

edittorrent 命令可以编辑（修改）.torrent 文件里的信息。

示例：

```
# 批量修改 .torrent 文件里的 tracker 地址（Announce 字段）
ptool edittorrent --update-tracker "https://..." *.torrent

# 查看命令帮助了解其更多用法
ptool edittorrent -h
```

### 拆包下载 (partialdownload)

该命令的设计目的不是用于刷流。而是用于使用 VPS 等硬盘空间有限的云服务器(分多次)下载体积非常大的单个种子，然后配合 [rclone][] 将下载的文件直接上传到云存储。

使用方法：

```
# 使用本命令前，将种子以暂停状态添加到客户端里
# 将客户端的某个种子内容的所有文件按 1TiB 切成几块，显示分片信息。
ptool partialdownload <client> <infoHash> --chunk-size 1TiB -a

# 设置客户端只下载该种子第 0 块切片(0-indexed)的内容。
ptool partialdownload <client> <infoHash> --chunk-size 1TiB --chunk-index 0

# 也可以用于跳过种子里特定文件。查看命令帮助了解更多用法。
ptool partialdownload <client> <infohash> --exclude "*.txt"
```

### 手动添加辅种种子到客户端 (xseedadd)

```
ptool xseedadd <client> <torrentFileNameOrIdOrUrl>...
```

xseedadd 命令将提供的种子作为辅种种子添加到客户端。程序将在客户端里寻找与提供的种子元信息（文件名、文件大小）完全一致的目标种子，然后将提供的种子作为目标种子的辅种添加到客户端。如果客户端里没有找到匹配的目标种子，程序不会添加提供的种子到客户端。"xseedadd" 命令添加的辅种种子会打上 `_xseed` 标签。

### 查找下载目录里的未做种文件 (findalone)

```
ptool findalone <client> <save-path>...
```

findalone 命令可以扫描并列出下载目录(save path)里所有当前未在 BitTorrent 客户端里做种的文件。可以提供多个 save-path。只有 save path 文件夹自身里的文件会被检查（不会递归读取子级目录）。会将找到的"孤立"文件(或文件夹)的完整路径输出到 stdout。

如果 ptool 运行在宿主机而 BitTorrent 客户端运行在 Docker 里，使用 `--map-save-path` 参数指定两者路径的映射关系。

如果指定 `--all` 参数，会显示下载目录里所有文件以及每个文件对应的客户端里的种子个数。

示例：

```
ptool findalone local D:\Downloads E:\Downloads F:\Downloads

ptool findalone local --map-save-path "/root/Downloads:/Downloads" /root/Downloads
```

### 标记 BT 客户端里 Tracker 状态异常的种子 (markinvalidtracker)

示例：

```
ptool markinvalidtracker <client> _all
```

markinvalidtracker 命令将在客户端里查找当前 Tracker 状态异常的种子，然后为这些种子打上 `_invalid_tracker` 标签。

markinvalidtracker 命令将以下几种情形认为是 Tracker 状态异常：

- 种子在 Tracker 未注册或已经被删除。
- 种子的 Tracker url 的 Passkey 或 Authkey 不正确。
- 种子超过了站点的同时下载/做种客户端数量上限。

markinvalidtracker 不会标记因为网络或站点服务器问题而当前无法连通 Tracker 的种子。

### 修改本地 BT 客户端里的种子内容文件保存路径 (movesavepath)

假设 BT 客户端里有一个种子的内容文件夹(content-path)路径是 `/root/Downloads/[BDRip]Clannad`，并且这个文件夹下存在不属于这个种子的其他文件（例如媒体库管理软件刮削生成的元文件 metainfo.nfo）：

```
/root/Downloads/[BDRip]Clannad
----- 01.mkv
----- 02.mkv
----- metainfo.nfo
```

如果在 qBittorrent 里直接用 "Set location" 功能将种子的保存路径(save-path)由 `/root/Downloads` 修改为 `/var/Downloads`，则只有属于该种子的文件会被移动，而其他文件（包括种子的原内容文件夹本身）仍然会存在于原路径：

```
/root/Downloads/[BDRip]Clannad
----- metainfo.nfo

/var/Downloads/[BDRip]Clannad
----- 01.mkv
----- 02.mkv
```

为了解决这个问题，ptool 提供一个 `movesavepath` 命令，能够将“整个内容文件夹”整体移动到其他路径，例如：

```
ptool movesavepath --client local /root/Downloads /var/Downloads
```

以上命令将 /root/Downloads 路径(old-save-path)里的种子内容移动到 /var/Downloads 路径(new-save-path)里，并且相应修改 local 客户端里相关种子的保存路径。该命令的内部工作方式是先导出并删除客户端里的原始种子（不删除硬盘文件）、移动 old-save-path 里内容到 new-save-path 里，然后再将之前导出的种子重新添加回客户端。

默认只会移动 old-save-path 里在 BT 客户端里存在相应种子的文件。如果指定 `--all` 参数，则 old-save-path 里所有文件都会被移动。

该命令只支持本地的 BT 客户端。如果 ptool 工作在宿主机而 BT 客户端位于 Docker 容器里，需要使用 `--map-save-path local_path:client_path` 参数指定宿主机与 Docker 容器里路径之间的映射关系，例如：

```
ptool movesavepath --client local /root/Downloads/Uncategoried /root/Downloads/Others --map-save-path "/root/Downloads:/Downloads"
```

### 同步 Cookies & 导入站点 (cookiecloud)

程序支持通过 [CookieCloud][] 服务器同步站点 Cookies 或导入站点。

要使用此功能，在 ptool.toml 配置文件里添加 CookieCloud 服务器连接信息：

```
[[cookieclouds]]
#name = "" # 名称可选
server = "https://cookiecloud.example.com"
uuid = "uuid"
password = "password"
```

可以添加任意个 CookieCloud 连接信息。如果想要让某个 CookieCloud 连接信息仅用于同步特定站点 cookies，加上 `sites = ["sitename"]` 这行配置。

#### 测试 CookieCloud 服务 (status)

```
ptool cookiecloud status
```

使用配置的 CookieCloud 连接信息连接服务器，测试配置正确性和当前服务器状态。

#### 同步站点 Cookies (sync)

```
ptool cookiecloud sync
```

程序会从 CookieCloud 服务器获取最新的 Cookies，并更新 ptool.toml 里已配置的站点的 Cookies。程序会对 ptool.toml 文件里的站点的当前 Cookie 和其从 CookieCloud 服务器获取的新版 Cookie 分别进行测试，只有在当前 Cookie 失效并且新版 Cookie 有效的情形才会更新 ptool.toml 里的站点 Cookie 字段值。

#### 导入站点 (import)

```
ptool cookiecloud import
```

程序会从 CookieCloud 服务器获取最新的 Cookies，筛选出本程序内置支持的站点(`ptool sites`)中当前 ptool.toml 文件里未配置、并且 CookieCloud 服务器数据里存在对应网站有效 Cookie 的站点，然后添加这些站点的配置信息到 ptool.toml 文件里。

import 命令不会检测或更新 ptool.toml 里当前已存在相应配置的站点的 Cookies。

#### 查看 CookieCloud 里的网站 Cookie (get)

```
ptool cookiecloud get <site>...
```

显示 CookieCloud 服务器数据里网站的最新 Cookies。参数可以是站点名、分组名、任意域名或 Url。

默认以 Http 请求 "Cookie" 头格式显示 Cookies。如果指定 `--format js` 参数，则会以 JavaScript 的 "document.cookie='';" 代码段格式显示 Cookies，可以直接将输出结果复制到浏览器 F12 开发者工具 Console 里执行以导入 Cookies。

### 查看内置支持站点信息 (sites)

```
# 显示所有内置支持的站点列表。ptool.toml 配置文件里将 [[sites]] 配置块的 type 设为站点的 Type 或 Alias。
ptool sites

# 显示对应站点在本程序内部使用的详细配置参数。参数为站点的 Type 或 Alias。
ptool sites show mteam
```

### 交互式终端 (shell)

`ptool shell` 可以启动一个交互式的 shell 终端环境。终端里可以运行所有 ptool 支持的命令。命令和命令参数输入支持完整的自动补全。

ptool 也支持 bash、powershell 等操作系统 shell 环境下的命令自动补全，需要在系统 shell 里安装程序生成的自动补全脚本。运行 `ptool completion` 了解详细信息。但由于技术限制，系统 shell 里仅支持基本的自动补全（不支持 BT 客户端名称、站点名称等动态内容参数的自动补全）。

### 站点种子信息显示

`status -t`, `batchdl`, `search` 等命令会将找到的站点种子以列表形式显示，示例：

```
Name Size Free Time ↑S ↓L ✓C ID P
You Sheng Xiao Shuo He Ji Mp3 M4a 265.3GiB ✓ 2022-10-31 06:08:14 14 1 61 redleaves.21 -
有声小说 大合集 Ⅸ - M4A 88.98GiB ✕ 2022-10-31 06:08:14 9 1 48 redleaves.20 -
忘尘阁 - 海的温度 - 演播悦库时光 - 完结 2.47GiB $✓(2d8h)       2023-08-21 14:37:14    36     0    53       redleaves.87849   -
教父一 - 马里奥·普佐 - 演播读客熊猫君 -   534.4MiB  $✕             2023-04-28 06:08:14   111     0   211       redleaves.14459   -
不灭龙帝 - 妖夜 - 演播何其 - 完结 - 2020  23.61GiB  2.0$✓(2d11h) 2023-08-14 18:08:14 89 2 110 redleaves.86125 -
```

列表里包括以下字段：

- Name : 种子名称。
- Size : 种子大小。
- Free : 种子的免费和其它关键属性信息。各个部分符号含义：
  - `2.0` : 上传量计算倍率。
  - `!` : HR 种子。
  - `$` : 付费(paid)种子。第一次下载或汇报种子时会扣除积分。
  - `✓` : 免费(free)种子（不计算下载量）。
  - `✕` : 非免费(non-free)种子（下载量倍率 > 0）。
  - `(1d12h)` : 种子优惠(下载量免费或折扣、上传量倍率等)剩余时间。
  - `N` : 中性(Neutral)种子。不计算上传量、下载量、做种魔力。
  - `Z` : 零流量种子。不计算上传量、下载量。
- Time : 种子的发布时间。
- ↑S : 种子的做种人数。
- ↓L : 种子的下载人数。
- ✓C : 种子的已完成人数。
- ID : 种子的 ID，包括站点名称前缀。可以用 `ptool add <client> <id>` 命令将种子添加到 BT 客户端。
- P : 种子的下载进度或历史下载状态。此信息由站点提供。目前显示的值为：
  - `-` : 未曾下载过此种子。
  - `✓` : 曾经下载或做种过此种子。
  - `*%` : 当前正在下载或做种此种子。

如果指定 `--dense` (`-d`) 参数，列表的 Name 列会输出完整名称，以及种子在站点里的“类别”和其它标签。

### 站点分组 (group) 功能

在 ptool.toml 配置文件里可以定义站点分组，例如：

```
[[groups]]
name = "acg"
sites = ["u2", "kamept"]
```

定义分组后，大部分命令中 `<site>` 类型的参数可以使用分组名代替以指代多个站点，例如：

```
# 在 acg 分组的所有站点中搜索 "clannad" 关键词的种子
ptool search acg clannad
```

预置的 `_all` 分组可以用来指代所有站点。

### 命令别名 (Alias) 功能

ptool.toml 里可以使用 `[[aliases]]` 区块自定义命令别名，例如：

```
[[aliases]]
name = "st"
cmd = "status local -t"
```

然后可以直接运行 `ptool st`, 等效于运行 `ptool status local -t`。

运行别名时也可以传入额外参数，并且支持指定额外参数中可选部分的默认值。例如：

```
[[aliases]]
name = "st"
cmd = "status -t"
minArgs = 0
defaultArgs = "local"
```

minArgs 是执行别名时必须传入的额外参数数量， defaultArgs 是额外参数可选部分的默认值。执行别名时，如果用户提供的额外参数数量 < minArgs ，程序会报错；如果用户提供的额外参数数量 == minArgs ，则 defaultArgs 会被追加到额外参数后面。定义以上别名后：

- 运行 `ptool st` 等效于运行 `ptool status -t local`
- 运行 `ptool st tr` 等效于运行 `ptool status -t tr`

说明：

- 定义的别名无法覆盖内置命令。
- 别名无法直接在 shell 里使用，可以使用 `ptool alias <name>` 在 shell 里执行别名。

### 模仿浏览器 (impersonate)

ptool 会在访问站点时自动模拟浏览器环境（类似 [curl-impersonate](https://github.com/lwthiker/curl-impersonate)），会设置 TLS ja3 指纹、HTTP2 akamai_fingerprint 指纹、访问请求的 http headers 等。测试能够绕过大多数站点的 CF 盾。

默认模仿最新稳定版 Chrome on Windows x64 en-US 环境。可以在 ptool.toml 里使用 `siteImpersonate = "chrome120"` 设置为想要模仿的浏览器。运行 `ptool version` 会列出所有支持模仿的浏览器环境列表。运行 `ptool version --show-impersonate chrome120` 查看对应模仿浏览器环境的详细参数。

[Private trackers]: https://wiki.installgentoo.com/wiki/Private_trackers
[BitTorrent]: https://en.wikipedia.org/wiki/BitTorrent
[CookieCloud]: https://github.com/easychen/CookieCloud
[IYUU]: https://github.com/ledccn/IYUUAutoReseed
[IYUU 接口]: https://api.iyuu.cn/docs.php
[IYUU 网站]: https://iyuu.cn/
[Reseed]: https://github.com/tongyifan/Reseed-backend
[Reseed 官网]: https://reseed.tongyifan.me/
[rclone]: https://github.com/rclone/rclone
[rclone lsjson]: https://rclone.org/commands/rclone_lsjson/
[松鼠党]: https://www.reddit.com/r/DataHoarder/
