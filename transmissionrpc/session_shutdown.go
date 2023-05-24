package transmissionrpc

import (
	"context"
	"fmt"
)

/*
	Session shutdown
	https://github.com/transmission/transmission/blob/3.00/extras/rpc-spec.txt#L615
*/

// SessionClose tells the transmission session to shut down.
// https://github.com/transmission/transmission/blob/3.00/extras/rpc-spec.txt#L619
func (c *Client) SessionClose(ctx context.Context) (err error) {
	// Send request
	if err = c.rpcCall(ctx, "session-close", nil, nil); err != nil {
		err = fmt.Errorf("'session-close' rpc method failed: %w", err)
	}
	return
}
