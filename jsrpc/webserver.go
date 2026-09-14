//go:build js

package jsrpc

import (
	"fmt"
	"net/rpc"
	"os"

	"github.com/zugcat/zugoui/wsrpc/rpctypes"
)

// Server client stub for [wsrpc.Server]
type Server struct {
	*rpc.Client
}

// Observer implements the Observer interface to the server with the Action
type Observer struct {
	*Server
	Action string
}

// Action client side stub for function [wsrpc.Server.Action]
func (s Server) Action(action string) {
	go func() {
		if err := s.Call(
			"Server.Action",
			&rpctypes.ActionReq{
				Action: action,
			},
			nil,
		); err != nil {
			fmt.Fprintf(os.Stderr, "Action(%s:%v\n", action, err)
		}
	}()
}

// SetValue client stub for function [wsrpc.Server.SetValue]
func (o Observer) SetValue(key string, value any) {
	go func() {
		if err := o.Call(
			"Server.SetValue",
			&rpctypes.SetValueReq{
				Action: o.Action,
				Key:    key,
				Value:  value,
			},
			nil,
		); err != nil {
			fmt.Fprintf(os.Stderr, "SetValue(%s:%v):%v\n", key, value, err)
		}
	}()
}

// InsertValueAt stub for function [wsrpc.Server.InsertValueAt]
func (o Observer) InsertValueAt(at int, value any) {
	// unsupported
}

// RemoveValueAt stub for function [wsrpc.Server.RemoveValueAt]
func (o Observer) RemoveValueAt(at int) {
	// unsupported
}

// SetValueAt stub for function [wsrpc.Server.SetValueAt]
func (o Observer) SetValueAt(at int, value any) {
	// unsupported
}

// SetValueFor stub for function [wsrpc.Server.SetValueFor]
func (o Observer) SetValueFor(key string, value any) {
	// unsupported
}

// RemoveValueFor stub for function [wsrpc.Server.RemoveValueFor]
func (o Observer) RemoveValueFor(key string) {
	// unsupported
}
