package mtorrent

import "fmt"

const (
	TorrentSearchMode_Normal = "normal"
	TorrentSearchMode_Adult  = "adult"

	TorrentSearchDirection_Asc  = "ASC"
	TorrentSearchDirection_Desc = "DESC"

	// Torrent discount
	TorrentDiscount_Normal    = "NORMAL"
	TorrentDiscount_50Percent = "PERCENT_50" // 50%
	TorrentDiscount_70Percent = "PERCENT_70" // 30%
	TorrentDiscount_Free      = "FREE"       // Free
)

type ResponseCode struct {
	Message string `json:"message"`
	Code    Int64  `json:"code"`
}

type DownloadTokenResponse struct {
	ResponseCode
	Data string `json:"data"`
}

type TorrentSearchRequest struct {
	Mode          string   `json:"mode"`
	Categories    []string `json:"categories"`
	Visible       int      `json:"visible"`
	Keyword       string   `json:"keyword,omitempty"`
	PageNumber    int64    `json:"pageNumber"`
	PageSize      int      `json:"pageSize"`
	SortDirection string   `json:"sortDirection,omitempty"`
	SortField     string   `json:"sortField,omitempty"`
}

type TorrentSearchRequestOption func(*TorrentSearchRequest)

func WithKeyword(keyword string) TorrentSearchRequestOption {
	return func(r *TorrentSearchRequest) {
		r.Keyword = keyword
	}
}

func WithMode(mode string) TorrentSearchRequestOption {
	return func(r *TorrentSearchRequest) {
		r.Mode = mode
	}
}

func WithSortField(field string) TorrentSearchRequestOption {
	return func(r *TorrentSearchRequest) {
		r.SortField = field
	}
}

func WithPageNumber(pageNumber int64) TorrentSearchRequestOption {
	return func(r *TorrentSearchRequest) {
		r.PageNumber = pageNumber
	}
}

func WithSortDirection(desc bool) TorrentSearchRequestOption {
	return func(r *TorrentSearchRequest) {
		if desc {
			r.SortDirection = TorrentSearchDirection_Desc
		} else {
			r.SortDirection = TorrentSearchDirection_Asc
		}
	}
}

func NewTorrentSearchRequest(opts ...TorrentSearchRequestOption) TorrentSearchRequest {
	req := TorrentSearchRequest{
		Mode:       TorrentSearchMode_Normal,
		Visible:    1,
		PageNumber: 1,
		PageSize:   100,
	}
	for _, opt := range opts {
		opt(&req)
	}

	return req
}

type TorrentStatus struct {
	Discount        string `json:"discount"`
	DiscountEndTime *Time  `json:"discountEndTime"`
	Leechers        Int64  `json:"leechers"`
	Seeders         Int64  `json:"seeders"`
	Status          string `json:"status"`
}

type Torrent struct {
	Id               string        `json:"id"`
	CreateDate       *Time         `json:"createdDate"`
	LastModifiedDate *Time         `json:"lastModifiedDate"`
	Name             string        `json:"name"`
	Description      string        `json:"smallDescr"`
	Category         string        `json:"category"`
	Size             Int64         `json:"size"`
	Status           TorrentStatus `json:"status"`
}

type TorrentList struct {
	PageNumber Int64     `json:"pageNumber"`
	PageSize   Int64     `json:"pageSize"`
	Total      Int64     `json:"total"`
	TotalPages Int64     `json:"totalPages"`
	Data       []Torrent `json:"data"`
}

type TorrentSearchResponse struct {
	ResponseCode
	Data TorrentList `json:"data"`
}

type Profile struct {
	Id               string `json:"id"`
	CreateDate       Time   `json:"createdDate"`
	LastModifiedDate Time   `json:"lastModifiedDate"`
	UserName         string `json:"username"`
	MemberCount      struct {
		Bonus      Int64String   `json:"bonus"`
		Uploaded   Int64String   `json:"uploaded"`
		Downloaded Int64String   `json:"downloaded"`
		ShareRate  Float64String `json:"shareRate"`
	} `json:"memberCount"`
}

type ProfileResponse struct {
	ResponseCode
	Data Profile `json:"data"`
}

type errorGetter interface {
	GetError() error
}

func (r ResponseCode) GetError() error {
	if r.Code != 0 {
		return fmt.Errorf("response error(%d): %s", r.Code, r.Message)
	}
	return nil
}
