//go:build js

package browser

import (
	"context"
	"fmt"
	"net/rpc"
	"os"
	"time"

	"github.com/coder/websocket"

	"github.com/zugcat/zugoui/jsrpc"
	"github.com/zugcat/zugoui/wsconn"
)

func connection(
	ctx context.Context,
	url string,
	server *jsrpc.Server,
	rpcServer *rpc.Server,
	ready func(),
) error {
	ws, _, err := websocket.Dial(ctx, url, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "websocket.Dial(%s) failed\n", url)
		return err
	}
	defer ws.CloseNow()

	pingCtx, stopPinging := context.WithCancel(ctx)
	defer stopPinging()
	go func() {
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if err := ws.Ping(pingCtx); err != nil {
					fmt.Fprintf(os.Stderr, "ping: %v\n", err)
					return
				}
			case <-pingCtx.Done():
				return
			}
		}
	}()

	// create a Demux for the two endpoints
	dmx := wsconn.NewDemux(ctx, wsconn.NewMessage(ctx, ws))
	defer dmx.Close()

	ep0 := dmx.NewEndpoint(0) // outgoing to web server
	ep1 := dmx.NewEndpoint(1) // incoming to browswer
	defer ep0.Close()
	defer ep1.Close()

	server.Client = rpc.NewClient(ep0)

	if ready != nil {
		ready()
	}

	rpcServer.ServeConn(ep1)
	return nil
}
