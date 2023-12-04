package cookiecloud

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/sagan/ptool/util"
	"github.com/sagan/ptool/util/crypto"
)

type CookieCloudBody struct {
	Uuid      string `json:"uuid,omitempty"`
	Encrypted string `json:"encrypted,omitempty"`
}

type CookiecloudData struct {
	// host => [{name,value,domain}...]
	Cookie_data map[string][]map[string]any `json:"cookie_data"`
}

func GetCookiecloudData(server string, uuid string, password string) (*CookiecloudData, error) {
	if server == "" || uuid == "" || password == "" {
		return nil, fmt.Errorf("all params of server,uuid,password must be provided")
	}
	if !strings.HasSuffix(server, "/") {
		server += "/"
	}
	var data *CookieCloudBody
	err := util.FetchJson(server+"get/"+uuid, &data, nil, "", "", nil)
	if err != nil || data == nil {
		return nil, fmt.Errorf("failed to get cookiecloud data: err=%v, null data=%t", err, data == nil)
	}
	keyPassword := crypto.Md5String(uuid, "-", password)[:16]
	decrypted, err := crypto.DecryptCryptoJsAesMsg(keyPassword, data.Encrypted)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt data: err=%v", err)
	}
	var cookiecloudData *CookiecloudData
	err = json.Unmarshal(decrypted, &cookiecloudData)
	if err != nil || cookiecloudData == nil {
		return nil, fmt.Errorf("failed to parse decrypted data as json: err=%v", err)
	}
	return cookiecloudData, nil
}
