//go:build js

package jsrpc

import (
	"fmt"
	"net/rpc"
	"os"
	"sync"

	"github.com/zugcat/zugoui/wsrpc/rpctypes"
)

// Server client stub for [wsrpc.Server]
type Server struct {
	*rpc.Client

	// sets are the values on their way to the server, which an action waits
	// for: a page that sets a value and calls an action straight away means
	// the action to see it, but net/rpc runs each call on its own goroutine.
	sets pendingSets
}

// pendingSets counts calls in flight. Adding never blocks, since the page
// sets values from the JS event loop, which the calls' answers need.
type pendingSets struct {
	lck  sync.Mutex
	idle *sync.Cond
	n    int
}

func (p *pendingSets) add() {
	p.lck.Lock()
	p.n++
	p.lck.Unlock()
}

func (p *pendingSets) done() {
	p.lck.Lock()
	p.n--
	if p.n == 0 && p.idle != nil {
		p.idle.Broadcast()
	}
	p.lck.Unlock()
}

// wait returns once no call is in flight
func (p *pendingSets) wait() {
	p.lck.Lock()
	defer p.lck.Unlock()

	if p.idle == nil {
		p.idle = sync.NewCond(&p.lck)
	}
	for p.n > 0 {
		p.idle.Wait()
	}
}

// Observer implements the Observer interface to the server with the Action
type Observer struct {
	*Server
	Action string
}

// Action client side stub for function [wsrpc.Server.Action]
func (s *Server) Action(action string) {
	go func() {
		s.sets.wait()
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

// CallAction client side stub for function [wsrpc.Server.Call].
// It blocks until the server answers, so call it off the JS event loop.
func (s *Server) CallAction(action string, args []byte) (*rpctypes.CallRes, error) {
	s.sets.wait()

	res := &rpctypes.CallRes{}
	if err := s.Client.Call(
		"Server.Call",
		&rpctypes.CallReq{
			Action: action,
			Args:   args,
		},
		res,
	); err != nil {
		return nil, err
	}
	return res, nil
}

// SetValue client stub for function [wsrpc.Server.SetValue]
func (o Observer) SetValue(key string, value any) {
	o.sets.add()
	go func() {
		defer o.sets.done()

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
