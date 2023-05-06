package tnode

// https://zhuque.in/api/user/getMainInfo
type apiMainInfoResponse struct {
	Status int64 `json:"status"`
	Data   struct {
		Username string `json:"username"`
		Download int64  `json:"download"`
		Upload   int64  `json:"upload"`
	} `json:"data"`
}
