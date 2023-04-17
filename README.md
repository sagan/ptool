# ptool

自用的 PT (private tracker) 网站辅助工具。提供全自动刷流(brush)、BT客户端控制等功能。

主要特性：

* 使用 Go 开发的纯 CLI 程序。单文件可执行程序，没有外部依赖。
* 无状态(stateless)：程序自身不保存任何状态、不在后台持续运行。“刷流”等任务需要使用 cron job 等方式定时运行本程序。
* 使用简单。只需5分钟时间，配置 BT 客户端地址、PT 网站地址和 cookie 即可开始全自动刷流。
* 目前支持的 BT 客户端： qBittorrent 4.1+。
* 目前支持的 PT 站点：绝大部分使用 nexusphp 的网站。
  * 测试过支持的站点：M-Team(馒头)、柠檬、U2、冬樱、红叶、聆音、铂金家、若干不可说的站点。
  * 未列出的大部分 np 站点应该也支持。除了个别魔改 np 很厉害的站点（例如城市）尚不支持。
* 刷流任务：
  * 不依赖 RSS。直接抓取站点页面上最新的种子。
  * 无需配置选种规则。自动跳过非免费的种子；自动筛选适合刷流的种子。
  * 无需配置删种规则。自动删除已无刷流价值的种子；硬盘空间不足时也会自动删种。

## 快速开始（刷流）

下载本程序的可执行文件 ptool (Linux) 或 ptool.exe (Windows) 放到任意目录，在同目录下创建名为 "ptool.toml" 的配置文件，内容示例如下：

```toml
[[clients]]
name = "local"
type = "qbittorrent" # 客户端类型。目前只支持 qbittorrent。需要启用 Web UI。
url = "http://localhost:8080/" # qBittorrent web UI 地址
username = "admin" # QB Web UI 用户名
password = "adminadmin" # QB Web UI 密码

[[sites]]
name = "mteam"
type = "nexusphp" # 站点类型。目前只支持 nexusphp
url = "https://kp.m-team.cc/" # 站点首页 URL
cookie = "cookie_here" # 浏览器 F12 获取的网站 cookie

```

可以使用 ```[[clients]]``` 和 ```[[sites]]``` 区块添加任意多个 BT 客户端和站点。

然后在当前目录下运行 ```ptool brush local mteam``` 即可执行刷流任务。程序会从 M-Team 获取最新的种子、根据一定规则筛选出适合的种子添加到本地的 qBittorrent 客户端里，同时自动从 BT 客户端里删除（已经没有上传的）旧的刷流种子。刷流任务添加到客户端里的种子会放到 ```_brush``` 分类(Category)里。程序只会对这个分类里的种子进行管理或删除等操作。

使用 cron job / 计划任务等方式定时执行上面的刷流任务命令（例如每隔 10 分钟执行一次）即可。



## 配置文件

程序支持使用 toml 或 yaml 格式的配置文件（```ptool.toml``` 或 ```ptool.toml```）。

配置文件可以之间放到程序启动时的当前目录下，也可以选择放到当前操作系统用户主目录下的 ".config/ptool/" 路径下（推荐）：

* Linux: ```~/.config/ptool/ptool.toml```
* Windows: ```%USERPROFILE%\.config\ptool\ptool.toml```

也可以通过启动程序时传入命令行参数 ```--config ptool.toml``` 手动指定使用的配置文件路径。

参考程序代码根目录下的 ```ptool.example.toml``` 和 ```ptool.example.yaml``` 示例配置文件了解所有可用的配置项。


## 程序功能

所有功能通过启动程序时传入的第一个”命令“参数区分：

```
ptool <command> args...
```

支持的 &lt;command&gt; :

* brush : 刷流。
* clientctl : BT 客户端控制。
* status : 显示 BT 客户端或 PT 站点当前状态信息。
* stats : 显示刷流任务流量统计。

### 刷流 (brush)

```
ptool brush <client> <site>... [flags]
```

刷流任务从指定的站点获取最新种子，选择适当的种子加入 BT 客户端；并自动从客户端中删除旧的（已没有上传速度的）刷流任务种子及其文件。刷流任务的目标是使 BT 客户端的上传速度达到软件中设置的上传速度上限（如果客户端里没有设置上传速度上限，本程序默认使用 10MB/s 这个值），如果当前 BT 客户端的上传速度已经达到或接近了上限（不管上传是否来源于刷流任务添加的种子），程序不会添加任何新种子。

参数

* &lt;client&gt; : 配置文件里定义的 BT 客户端 name。
* &lt;site&gt; : 配置文件里定义的 PT 站点 name。

可以提供多个 &lt;site&gt; 参数。程序会按随机顺序从提供的 &lt;site&gt; 列表里的各站点获取最新种子、筛选并选取一定数量的合适的种子添加到 BT 客户端。可以将同一个站点名重复出现多次以增加其权重，使刷流任务添加该站点种子的几率更大。

示例

```
# 使用 local 这个 BT 客户端，刷流 mteam 站点
ptool brush local mteam
```

选种（选择新种子添加到 BT 客户端）规则：

* 不会选择有以下任意特征的种子：不免费、存在 HnR 考查、免费时间临近截止。
* 发布时间过久的种子也不会被选择。
* 种子的当前做种、下载人数，种子大小等因素也都会考虑。

删种（删除 BT 客户端里旧的刷流种子）规则：

* 未下载完成的种子免费时间临近截止时，删除种子或停止下载（只上传模式）。
* 硬盘剩余可用空间不足（默认保留 5GB）时，开始删除没有上传速度的种子。
* 未下载完成的种子，如果长时间没有上传速度或上传/下载速度比例过低，也可能被删除。


刷流任务添加到客户端里的种子会放到 ```_brush``` 分类里。程序只会对这个分类里的种子进行管理或删除等操作。不会干扰 BT 客户端里其它正常的下载任务。如果需要永久保留某个刷流任务添加的种子（防止其被自动删除），在 BT 客户端里更改其分类即可。

### BT 客户端控制 (clientctl)

```
ptool clientctl <client> [<option>[=value] ...]
```

clientctl 命令可以显示或修改指定 name 的 BT 客户端的配置参数。


支持的参数(&lt;option&gt;) 列表：

* global_download_speed_limit : 全局下载速度上限。
* global_upload_speed_limit : 全局上传速度上限。

示例：

```
# 获取 local 客户端所有参数当前值
ptool clientctl local

# 设置 local 客户端的全局上传速度限制为 10MB/s
ptool clientctl local global_upload_speed_limit=10M
```

### 显示 BT 客户端或 PT 站点状态 (status)

```
ptool status <clientOrSite>...
```

显示指定 name 的 BT 客户端或 PT 站点的当前状态信息。可以提供多个名称。

显示的信息包括：

* BT 客户端：显示当前下载 / 上传速度和其上限，硬盘剩余可用空间。
* PT 站点：显示用户名、上传量、下载量。

### 显示刷流任务流量统计 (stats)

```
ptool stats [client...]
```

显示 BT 客户端的刷流任务流量统计信息（下载流量、上传流量总和）。本功能默认不启用，如需启用，在 ptool.yaml 配置文件的最上方里增加一行：```brushEnableStats: true``` 配置项。启用刷流统计后，刷流任务会使用 ptool.yaml 配置文件相同目录下的 "ptool_stats.txt" 文件存储所需保存的信息。

只有刷流任务添加和管理的 BT 客户端的种子（即 ```_brush``` 分类的种子）的流量信息会被记录和统计。目前设计只有在刷流任务从 BT 客户端删除某个种子时才会记录和统计该种子产生的流量信息。