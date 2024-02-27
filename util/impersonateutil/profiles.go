package impersonateutil

import (
	"github.com/sagan/ptool/util"
)

// may contain too long (width > 120) lines

const (
	// 默认对站点的 http 请求模仿最新版 Chrome Windows 11 x64 en-US 环境
	DEFAULT_IMPERSONATE = "chrome120"
)

// 对站点的 http 请求模仿真实浏览器环境。包括：
// TLS Ja3 指纹、http2 指纹、http headers。
// 查看当前 http 客户端的 ja3, http2 指纹, http headers 等信息:
// https://tls.peet.ws/api/all (该网站生成的ja3可能有问题),
// https://tools.scrapfly.io/api/fp/anything ,
// https://scrapfly.io/web-scraping-tools/ja3-fingerprint (建议用这个 ja3).
var profiles = []*Profile{
	{
		Name:      "chrome120",
		Navigator: "chrome",
		Comment:   "Chrome 120 on Windows 11 x64 en-US",
		// TLS ja3 指纹。参考: https://scrapfly.io/blog/how-to-avoid-web-scraping-blocking-tls/ .
		// Ja3 should be generated without the "TLS Session has been resurected" warning
		Ja3: "772,4865-4866-4867-49195-49199-49196-49200-52393-52392-49171-49172-156-157-47-53,65281-45-11-65037-18-5-51-0-23-27-43-16-10-35-17513-13,29-23-24,0",
		// akamai_fingerprint 格式。http2 指纹参考: https://lwthiker.com/networks/2022/06/17/http2-fingerprinting.html .
		H2fingerpring: "1:65536,2:0,4:6291456,6:262144|15663105|0|m,a,s,p",
		// 请求默认 http headers。有序！
		Headers: [][]string{
			{"Cache-Control", "max-age=0"},
			{"Sec-Ch-Ua", `"Not_A Brand";v="8", "Chromium";v="120", "Google Chrome";v="120"`},
			{"Sec-Ch-Ua-Mobile", `?0`},
			{"Sec-Ch-Ua-Platform", `"Windows"`},
			{"Upgrade-Insecure-Requests", "1"},
			{"User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"},
			{"Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7"},
			{"Sec-Fetch-Site", `none`},
			{"Sec-Fetch-Mode", `navigate`},
			{"Sec-Fetch-User", `?1`},
			{"Sec-Fetch-Dest", `document`},
			{"Accept-Encoding", "gzip, deflate, br"},
			{"Accept-Language", "en-US,en;q=0.9"},
			{"Cookie", util.HTTP_HEADER_PLACEHOLDER},
		},
	},
	{
		Name:      "firefox121",
		Navigator: "firefox",
		Comment:   "Firefox 121 on Windows 11 x64 en-US",
		// Ja3:           "772,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-156-157-47-53,0-23-65281-10-11-35-16-5-34-51-43-13-45-28-65037,29-23-24-25-256-257,0",
		// utls do not support TLS 34 delegated_credentials (34) (IANA) extension at this time.
		// see https://github.com/refraction-networking/utls/issues/274
		Ja3:           "772,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-156-157-47-53,0-23-65281-10-11-35-16-5-51-43-13-45-28-65037,29-23-24-25-256-257,0",
		H2fingerpring: "1:65536,4:131072,5:16384|12517377|3:0:0:201,5:0:0:101,7:0:0:1,9:0:7:1,11:0:3:1,13:0:0:241|m,p,a,s",
		Headers: [][]string{
			{"User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:121.0) Gecko/20100101 Firefox/121.0"},
			{"Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8"},
			{"Accept-Language", "en-US,en;q=0.5"},
			{"Accept-Encoding", "gzip, deflate, br"},
			{"Cookie", util.HTTP_HEADER_PLACEHOLDER},
			{"Upgrade-Insecure-Requests", "1"},
			{"Sec-Fetch-Dest", `document`},
			{"Sec-Fetch-Mode", `navigate`},
			{"Sec-Fetch-Site", `none`},
			{"Sec-Fetch-User", `?1`},
			{"te", "trailers"},
		},
	},
}
