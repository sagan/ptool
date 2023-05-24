package transmissionrpc

import (
	"context"
	"fmt"
)

/*
	Removing a Torrent
	https://github.com/transmission/transmission/blob/3.00/extras/rpc-spec.txt#L428
*/

// TorrentRemove allows to delete one or more torrents only or with their data.
// https://github.com/transmission/transmission/blob/3.00/extras/rpc-spec.txt#L430
func (c *Client) TorrentRemove(ctx context.Context, payload TorrentRemovePayload) (err error) {
	// Send payload
	if err = c.rpcCall(ctx, "torrent-remove", payload, nil); err != nil {
		return fmt.Errorf("'torrent-remove' rpc method failed: %w", err)
	}
	return
}

// TorrentRemovePayload holds the torrent id(s) to delete with a data deletion flag.
// https://github.com/transmission/transmission/blob/3.00/extras/rpc-spec.txt#L432
type TorrentRemovePayload struct {
	IDs             []int64 `json:"ids"`
	DeleteLocalData bool    `json:"delete-local-data"`
}
