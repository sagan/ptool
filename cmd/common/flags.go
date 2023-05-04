package common

import (
	"errors"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

type SiteTorrentSortFieldEnum string

const SiteTorrentSortFieldEnumTip = `any of: size|time|name|seeders|leechers|snatched|none`
const SiteTorrentSortFieldEnumInvalidTip = `must be any of: size|time|name|seeders|leechers|snatched|none`

// String is used both by fmt.Print and by Cobra in help text
func (e *SiteTorrentSortFieldEnum) String() string {
	return string(*e)
}

// Set must have pointer receiver so it doesn't change the value of a copy
func (e *SiteTorrentSortFieldEnum) Set(v string) error {
	switch v {
	case "size", "time", "name", "seeders", "leechers", "snatched", "none":
		*e = SiteTorrentSortFieldEnum(v)
		return nil
	default:
		return errors.New(SiteTorrentSortFieldEnumInvalidTip)
	}
}

// Type is only used in help text
func (e *SiteTorrentSortFieldEnum) Type() string {
	return "sortField"
}

func SiteTorrentSortFieldEnumCompletion(cmd *cobra.Command, args []string, toComplete string) (
	[]string, cobra.ShellCompDirective) {
	return []string{
		"size\tsort by torrent size ",
		"time\tsort by torrent creation time ",
		"name\tsort by torrent name",
		"none\tdo not sort",
	}, cobra.ShellCompDirectiveDefault
}

type OrderEnum string

const OrderEnumTip = `any of: asc|desc`
const OrderEnumInvalidTip = `must be any of: asc|desc`

// String is used both by fmt.Print and by Cobra in help text
func (e *OrderEnum) String() string {
	return string(*e)
}

// Set must have pointer receiver so it doesn't change the value of a copy
func (e *OrderEnum) Set(v string) error {
	switch v {
	case "asc", "desc":
		*e = OrderEnum(v)
		return nil
	default:
		return errors.New(OrderEnumInvalidTip)
	}
}

// Type is only used in help text
func (e *OrderEnum) Type() string {
	return "orderType"
}

func OrderEnumCompletion(cmd *cobra.Command, args []string, toComplete string) (
	[]string, cobra.ShellCompDirective) {
	return []string{
		"asc\tby asc order",
		"desc\tby desc order",
	}, cobra.ShellCompDirectiveDefault
}

/*
Usage:
var sortFieldEnumFlag SiteTorrentSortFieldEnum
myCmd.Flags().VarP(&sortFieldEnumFlag, "sort", "", `Sort field. allowed: "size", "name", "none"`)
myCmd.RegisterFlagCompletionFunc("sort", SiteTorrentSortFieldEnumCompletion)
*/

var (
	_ pflag.Value = (*SiteTorrentSortFieldEnum)(nil)
)
