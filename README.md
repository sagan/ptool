# ptool

自用的 PT (private tracker) 网站辅助工具。提供全自动刷流(brush)、BT客户端控制等功能。

主要特性：

* 使用 Go 开发的纯 CLI 程序。单文件可执行程序，没有外部依赖。
* 无状态(stateless)：程序自身不保存任何状态、不在后台持续运行。“刷流”等任务需要使用 cron job 等方式定时运行本程序。
* 使用简单。只需配置 BT 客户端地址、PT 网站地址和 cookie 等信息即可开始刷流。
* 目前支持的 BT 客户端： qBittorrent 4.1+。
* 目前支持的 PT 站点：绝大部分使用 nexusphp 的网站。
  * 测试过支持的站点：M-Team(馒头)、若干不可说（或不明确是否可说）的站点。
  * 个别魔改 np 很厉害的站点尚不支持。

## 快速开始（刷流）

下载本程序的可执行文件 ptool (Linux) 或 ptool.exe (Windows) 放到任意目录，在这个目录下创建名为 "ptool.yaml" 的配置文件，内容示例如下：

```yaml
clients: # BT 客户端列表
  -
    name: "local"
    type: "qbittorrent" # 客户端类型。目前只支持 qbittorrent
    url: "http://localhost:8085/" # qBittorrent web UI 地址
    username: "admin" # QB Web UI 用户名
    password: "adminadmin" # QB Web UI 密码
sites: # PT 网站列表
  -
    name: "mteam"
    type: "nexusphp" # 站点类型。目前只支持 nexusphp
    url: "https://kp.m-team.cc/" # 站点首页 URL
    cookie: "cookie_here" # 浏览器 F12 获取的网站 cookie
```

然后在当前目录下运行 ```ptool brush local mteam``` 即可执行刷流任务。程序会从 M-Team 获取最新的种子、根据一定规则筛选出适合的种子添加到本地的 qBittorrent 下载器里，同时自动从 BT 客户端里删除（已经没有上传的）旧的刷流种子。刷流任务添加到下载器里的种子会放到 ```_brush``` 分类(Category)里。程序只会对这个分类里的种子进行管理或删除等操作。

使用 cron job / 计划任务等方式定时执行上面的刷流任务命令（例如每隔 10 分钟执行一次）即可。

您也可以选择把 ```ptool.yaml``` 配置文件放到当前操作系统用户主目录下的 ".config/ptool/" 路径下：

* Linux: ```~/.config/ptool/ptool.yaml```
* Windows: ```%USERPROFILE%\.config\ptool\ptool.yaml```

## 程序功能

所有功能通过启动程序时传入的第一个参数区分：

```
ptool <command> ...args
```

支持的 command :

* brush : 刷流。
* clientctl : BT 客户端控制。
* status : 显示 BT 客户端或 PT 站点当前状态信息。

### 刷流 (brush)

```
ptool brush <client> <site> [flags]
```

* <client> : 配置文件里定义的 BT 客户端 name。
* <site> : 配置文件里定义的 PT 站点 name。

### BT 客户端控制 (clientctl)

```
# 获取客户端所有参数当前值
ptool clientctl <client>

# 获取客户端指定参数当前值
ptool clientctl <client> <option>

# 设置客户端参数
ptool clientctl <client> <option>=<value>
```

参数(&lt;option&gt;) 列表：

* global_download_speed_limit : 全局下载速度限制。
* global_upload_speed_limit : 全局上传速度限制。

例如：

```
# 设置 local 客户端的上传速度限制为 10MB/s
ptool clientctl local global_upload_speed_limit=10M
```