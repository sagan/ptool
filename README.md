# ptool

自用的 PT (private tracker) 网站辅助工具([Github](https://github.com/sagan/ptool))。提供全自动刷流(brush)、自动辅种(使用 iyuu 接口)、BT 客户端控制等功能。

主要特性：

- 使用 Go 开发的纯 CLI 程序。单文件可执行程序，没有外部依赖。支持 Windows / Linux、x64 / arm64 等多种环境、架构。
- 无状态(stateless)：程序自身不保存任何状态、不在后台持续运行。“刷流”等任务需要使用 cron job 等方式定时运行本程序。
- 使用简单。只需 5 分钟时间，配置 BT 客户端地址、PT 网站地址和 cookie 即可开始全自动刷流。
- 目前支持的 BT 客户端： qBittorrent v4.1+ / Transmission (<= v3.0)。
  - 推荐使用 qBittorrent。Transmission 客户端未充分测试。
- 目前支持的 PT 站点：绝大部分使用 nexusphp 的网站。
  - 测试过支持的站点：M-Team(馒头)、柠檬、U2、冬樱、红叶、聆音、铂金家、若干不可说的站点。
  - 未列出的大部分 np 站点应该也支持。除了个别魔改 np 很厉害的站点可能不支持。
- 刷流功能(brush)：
  - 不依赖 RSS。直接抓取站点页面上最新的种子。
  - 无需配置选种规则。自动跳过非免费的和有 HR 的种子；自动筛选适合刷流的种子。
  - 无需配置删种规则。自动删除已无刷流价值的种子；自动删除免费时间到期并且尚未下载完成的种子；硬盘空间不足时也会自动删种。
- 自动模仿浏览器访问 PT 站点，能够绕过大多数站点的 CF 盾 (impersonate 特性)。

## 快速开始（刷流）

下载本程序的可执行文件 ptool (Linux) 或 ptool.exe (Windows) 放到任意目录，在同目录下创建名为 "ptool.toml" 的配置文件，内容示例如下：

```toml
[[clients]]
name = "local"
type = "qbittorrent" # 客户端类型。目前主要支持 qBittorrent v4.1+。需要启用 Web UI
url = "http://localhost:8080/" # qBittorrent web UI 地址
username = "admin" # QB Web UI 用户名
password = "adminadmin" # QB Web UI 密码

[[sites]]
type = "mteam"
cookie = "cookie_here" # 浏览器 F12 获取的网站 cookie
```

然后在当前目录下运行 `ptool brush local mteam` 即可执行刷流任务。程序会从 M-Team 获取最新的种子、根据一定规则筛选出适合的种子添加到本地的 qBittorrent 客户端里，同时自动从 BT 客户端里删除（已经没有上传的）旧的刷流种子。刷流任务添加到客户端里的种子会放到 `_brush` 分类(Category)里。程序只会对这个分类里的种子进行管理或删除等操作。

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
#name = "mteam" # (可选)手动指定站点名称。如果不指定，默认使用其 type 作为 name
type = "mteam"
cookie = "cookie_here" # 浏览器 F12 获取的网站 cookie

# 方式 2：使用通用的 nexusphp 等站点架构类型，需要手动指定站点名称(name)、站点 url 和其他参数。
[[sites]]
name = "mteam"
type = "nexusphp" # 通用站点架构类型。可选值: nexusphp|gazellepw|unit3d|tnode|discuz
url = "https://kp.m-team.cc/" # 站点首页 URL
cookie = "cookie_here" # 浏览器 F12 获取的网站 cookie
```

推荐使用“方式 1”。程序内置了对大部分国内 NexusPHP PT 站点的支持。站点 type 通常为 PT 网站域名的主体部分（不含次级域名和 TLD 部分），例如 BTSCHOOL ( https://pt.btschool.club/ )的站点 type 是 btschool。部分 PT 网站也可以使用别名(alias)配置，例如 M-TEAM ( https://kp.m-team.cc/ )在本程序配置文件里的 type 设为 "m-team" 或 "mteam" 均可。运行 `ptool sites` 查看所有本程序内置支持的 PT 站点列表。本程序没有内置支持的 PT 站点必须通过“方式 2”配置。 （注：部分非 NP 架构站点本程序目前只支持自动辅种、查看站点状态，暂不支持刷流、搜索站点种子等功能）

配置好站点后，使用 `ptool status <site> -t` 测试（`<site>`参数为站点的 name）。如果配置正确且 Cookie 有效，会显示站点当前登录用户的状态信息和网站最新种子列表。

程序支持自动与浏览器同步站点 Cookies 或导入站点信息。详细信息请参考本文档 "cookiecloud" 命令说明部分。

参考程序代码根目录下的 `ptool.example.toml` 或 `ptool.example.yaml` 示例配置文件了解常用配置项信息。

查看程序代码 [config/config.go](https://github.com/sagan/ptool/blob/master/config/config.go) 文件里的 type ConfigStruct struct 获取全部可配置项信息。

## 程序功能

所有功能通过启动程序时传入的第一个”命令“参数区分：

```
ptool <command> args... [flags]
```

所有可用的 `<command>` 包括:

- brush : 自动刷流。
- iyuu : 使用 iyuu 接口自动辅种。
- batchdl : 批量下载站点的种子。
- status : 显示 BT 客户端或 PT 站点当前状态信息。
- stats : 显示刷流任务流量统计。
- search : 在某个站点搜索指定关键词的种子。
- add : 将种子添加到 BT 客户端。
- dltorrent : 下载站点的种子。
- BT 客户端控制命令集: clientctl / show / pause / resume / delete / reannounce / recheck / getcategories / createcategory / removecategories / setcategory / gettags / createtags / deletetags / addtags / removetags / edittracker / addtrackers / removetrackers / setsavepath。
- parsetorrent : 显示种子(torrent)文件信息。
- verifytorrent : 测试种子(torrent)文件与硬盘上的文件内容一致。
- partialdownload : 拆包下载。
- cookiecloud (v0.1.8+): 使用 [CookieCloud](https://github.com/easychen/CookieCloud) 同步站点的 Cookies 或导入站点。
- sites : 显示本程序内置支持的所有 PT 站点列表。
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

iyuu 命令通过 [iyuu 接口](https://api.iyuu.cn/docs.php) 提供自动辅种(cross seed)功能。本功能直接访问 iyuu 的服务器，本机上不需要安装 / 运行 iyuu 客户端。

#### iyuu 配置

如果是第一次使用 iyuu，首先需要在 [iyuu 网站](https://iyuu.cn/) 上微信扫码申请 IYUU 令牌（token）。在本程序的配置文件 ptool.toml 里配置 iyuu token：

```
iyuuToken = "IYUU0011223344..."
```

然后使用 iyuu 支持的任意合作站点的 uid 和 passkey 激活和绑定(bind) iyuu token，命令格式如下：

```
ptool iyuu bind --site zhuque --uid 123456 --passkey 0123456789abcdef
```

所有参数均必须提供

- --site : 用于验证的 PT 站点名。可以使用 `ptool iyuu sites -b` 命令查询 iyuu 支持的合作站点列表。
- --uid : 对应 PT 站点的用户 uid（数字）。在 PT 网站的个人页面获取。
- --passkey : 对应 PT 站点的用户 passkey。在 PT 网站的个人页面获取。

其他说明：

- 使用 `ptool iyuu sites -a` 查看 iyuu 支持的所有可辅种站点列表。
- 使用 `ptool iyuu status` 查询当前 iyuu token 的激活和绑定状态。

#### 使用 iyuu 自动辅种

```
ptool iyuu xseed <client>...
```

可以提供多个 client。程序会获取这些 client 里正在做种的种子信息，通过 iyuu 接口查询可以辅种的种子并将其自动添加到对应客户端里。注意只有在本程序的 ptool.toml 配置文件里添加的站点才会被辅种。

iyuu xseed 子命令支持很多可选参数。运行 `ptool iyuu xseed -h` 查看所有可选参数使用说明。

添加的辅种种子默认跳过客户端 hash 校验并立即开始做种。本程序会对客户端里目标种子和 iyuu 接口返回的候选辅种种子的文件列表进行比较（文件路径、大小），只有完全一致才会添加辅种种子。添加的辅种种子会打上 `_xseed` 标签。

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

`<infoHash>` 参数为指定的 BT 客户端里需要操作的种子的 infoHash 列表。也可以使用以下特殊值参数操作多个种子（delete 命令根据除 infoHash 以外的条件删除种子时需要二次确认）：

- \_all : 所有种子
- \_done : 所有已下载完成的种子（无论是否正在做种）(\_seeding | \_completed)
- \_undone : 所有未下载完成的种子(\_downloading | \_paused)
- \_active : 当前正在活动（上传或下载）的种子
- \_error : 状态为“出错”的种子
- \_downloading / \_seeding / \_paused / \_completed : 状态为正在下载 / 做种 / 暂停下载 / 下载完成(但未做种)的种子

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

# 从客户端删除指定种子（默认同时删除文件）
ptool delete local 31a615d5984cb63c6f999f72bb3961dce49c194a

# 特别的，如果 show 命令只提供一个 infoHash 参数，会显示该种子的所有详细信息。
ptool show local 31a615d5984cb63c6f999f72bb3961dce49c194a
```

#### 管理 BT 客户端里的的种子分类 / 标签 / Trackers 等(getcategories / createcategory / removecategories / setcategory / gettags / createtags / deletetags / addtags / removetags / edittracker / addtrackers / removetrackers / setsavepath)

```
# 获取所有分类
ptool getcategories <client>

# 新增分类(也可用于修改已有分类的下载目录)
ptool createcategory <client> <category> --save-path "/root/downloads"

# 删除分类
ptool removecategories <client> <category>...

# 修改种子的所属分类
ptool setcategory <client> <category> <infoHashes>...

# 获取所有标签(tag)
ptool gettags <client>

# 创建新的标签
ptool createtags <client> <tags>...

# 删除标签
ptool deletetags <client> <tags>...

# 为客户端里种子添加tag
ptool addtags <client> <tags> <infoHashes>...

# 为客户端里种子删除tag
ptool removetags <client> <tags> <infoHashes>...

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
```

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

### 下载站点的种子

```
ptool dltorrent <torrentIdOrUrl>...
```

将站点的种子文件下载到本地。参数是站点的种子 id 或 url（参考上面 "add" 命令）。

可选参数：

- --dir : 下载的种子文件保存路径。默认为当前目录(.)。

### 搜索 PT 站点种子 (search)

```
ptool search <sites> <keyword>
```

`<sites>` 参数为需要所搜索的 PT 站点，可以使用 "," 分割提供多个站点。可以使用 "\_all" 搜索所有已配置的 PT 站点。

可以用 `ptool add` 命令将搜索结果列表中的种子添加到 BT 客户端。

### 批量下载种子 (batchdl)

提供一个 batchdl 命令用于批量下载 PT 网站的种子（别名：ebookgod）。默认按种子体积大小升序排序、跳过死种和已经下载过的种子。

```
# 默认显示找到的种子列表
ptool batchdl <site>

# 下载找到的种子到当前目录
ptool batchdl <site> --action download


# 直接将种子添加到 "local" BT 客户端里
ptool batchdl <site> --action add --add-client local
```

此命令提供非常多的配置参数。部分常用参数：

- --max-torrents int : 最多下载多少个种子。默认 -1 (无限制，一直运行除非手动 Ctrl + C 停止)。
- --sort string : 站点种子排序方式：size|time|name|seeders|leechers|snatched|none (default size)
- --order string : 排序顺序：asc|desc。默认 asc。
- --min-torrent-size string : 种子大小的最小值限制 (e.g.: "100MiB", "1GiB")。默认为 "-1"（无限制）。
- --max-torrent-size string : 种子大小的最大值限制。默认为 "-1"（无限制）。
- --max-total-size string : 下载种子内容总体积最大值限制 (e.g.: "512GiB", "1TiB")。默认为 "-1"（无限制）。
- --free : 只下载免费种子。
- --no-hr : 跳过存在 HR 的种子。
- --no-paid : 跳过"付费"的种子。(部分站点存在"付费"种子，第一次下载或汇报时扣除积分)
- --base-url : 手动指定种子列表页 URL，例如："special.php"、"adult.php"、"torrents.php?cat=100"。

### 显示种子文件信息 (parsetorrent)

```
ptool parsetorrent file.torrent...
```

显示本地硬盘里的种子文件的元信息。

### 校验种子文件与硬盘内容是否一致 (verifytorrent)

示例：

```
ptool verifytorrent file.torrent --save-path D:\Downloads --check

ptool verifytorrent MyTorrent.torrent --content-path D:\Downloads\MyTorrent --check
```

参数

- `--save-path` : 种子内容保存路径(下载文件夹)。可以用于校验多个 torrent 文件。
- `--content-path` : 种子内容路径(root folder 或单文件种子的文件路径)。只能用于校验 1 个 torrent 文件。必须且只能提供 `--save-path` 和 `-content-path` 两者中的其中 1 个参数。
- `--check` : 对硬盘上文件进行 hash 校验。如果不提供此参数，默认只对比文件元信息(文件名、文件大小)。

### 拆包下载 (partialdownload)

使用方法：

```
# 使用本命令前，将种子以暂停状态添加到客户端里
# 将客户端的某个种子内容的所有文件按 1TiB 切成几块，显示分片信息。
ptool partialdownload <client> <infoHash> --chunk-size 1TiB -a

# 设置客户端只下载该种子第 0 块切片(0-indexed)的内容。
ptool partialdownload <client> <infoHash> --chunk-size 1TiB --chuck-index 0
```

该命令的设计目的不是用于刷流。而是用于使用 VPS 等硬盘空间有限的云服务器(分多次)下载体积非常大的单个种子，然后配合 rclone 将下载的文件直接上传到云盘。

### 同步 Cookies & 导入站点 (cookiecloud)

程序支持通过 [CookieCloud](https://github.com/easychen/CookieCloud) 服务器同步站点 Cookies 或导入站点。

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

status -t, batchdl, search 等命令会将找到的站点种子以列表形式显示，示例：

```
Name                                          Size  Free           Time                   ↑S    ↓L    ✓C                    ID   P
You Sheng Xiao Shuo He Ji Mp3 M4a         265.3GiB  ✓              2022-10-31 06:08:14    14     1    61          redleaves.21   -
有声小说  大合集Ⅸ - M4A                   88.98GiB  ✕              2022-10-31 06:08:14     9     1    48          redleaves.20   -
忘尘阁 - 海的温度 - 演播悦库时光 - 完结    2.47GiB  $✓(2d8h)       2023-08-21 14:37:14    36     0    53       redleaves.87849   -
教父一 - 马里奥·普佐 - 演播读客熊猫君 -   534.4MiB  $✕             2023-04-28 06:08:14   111     0   211       redleaves.14459   -
不灭龙帝 - 妖夜 - 演播何其 - 完结 - 2020  23.61GiB  2.0$✓(2d11h)   2023-08-14 18:08:14    89     2   110       redleaves.86125   -
```

列表里包括以下字段：

- Name : 种子名称。
- Size : 种子大小。
- Free : 种子的免费和其它关键属性信息。各个部分符号含义：
  - `2.0` : 上传量计算倍率。
  - `!` : HR 种子。
  - `$` : 付费(paid)种子。第一次下载或汇报种子时会扣除积分。
  - `✓` : 免费(free)种子（不计算下载量）。
  - `✕` : 非免费(none-free)种子（下载量倍率 > 0）。
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
  - `0%` : 正在下载或下载过此种子。

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

然后可以直接运行 `ptool st`, 等效于运行 `ptool status local -t`。注：定义的别名无法覆盖内置命令。

### 模仿浏览器 (impersonate)

ptool 会在访问站点时自动模拟浏览器环境（类似 [curl-impersonate](https://github.com/lwthiker/curl-impersonate)），会设置 TLS ja3 指纹、HTTP2 akamai_fingerprint 指纹、访问请求的 http headers 等。测试能够绕过大多数站点的 CF 盾。

默认模仿最新稳定版 Chrome on Windows x64 en-US 环境。可以在 ptool.toml 里使用 `siteImpersonate = "chrome120"` 设置为想要模仿的浏览器。运行 `ptool version` 会列出所有支持模仿的浏览器环境列表。运行 `ptool version --show-impersonate chrome120` 查看对应模仿浏览器环境的详细参数。
