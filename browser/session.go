//go:build js

package browser

import (
	"context"
	"errors"
	"net/rpc"
	"sync"
	"syscall/js"
	"time"

	"github.com/zugcat/zugoui/jsglue"
	"github.com/zugcat/zugoui/jsrpc"
	"github.com/zugcat/zugoui/jstypes"
	"github.com/zugcat/zugoui/wsrpc/rpctypes"
)

// The states a session reports to the page.
const (
	stateOpen         = "open"         // connected
	stateReconnecting = "reconnecting" // the connection dropped, and the session is redialing
	stateClosed       = "closed"       // closed by the page, or never connected
)

// Reconnecting backs off exponentially between these.
const (
	minRedial = 1 * time.Second
	maxRedial = 30 * time.Second
)

var errClosed = errors.New("zugoui: the connection closed before it opened")

// session is one websocket to one endpoint, as the page sees it:
//
//	state(): "open" | "reconnecting" | "closed"
//	subscribeState(cb: (state) => void): () => void
//	form(name: string): FormStore
//	action(name: string, args?: any): Promise<ActionResult>
//	close(): void
//
// The server's controller binds its models under form names; form(name)
// returns a store for one of them, which exists before the server binds it
// and keeps its subscribers when a dropped connection is redialed.
//
// action(name, args) calls an action the server handles with
// controller.HandleAction. It resolves to {ok: true, value} with the action's
// result, or {ok: false, error: {code, message, params}}. It never rejects;
// a call the connection drops resolves with the code "disconnected".
type session struct {
	url    string
	ctx    context.Context
	cancel context.CancelFunc
	funcs  jsglue.Funcs
	opened chan error // the first connection's outcome
	closed sync.Once

	lck       sync.Mutex
	state     string
	server    *jsrpc.Server // nil while not connected
	stores    map[string]*formStore
	stateSubs subscribers
	obj       map[string]any
}

func newSession(ctx context.Context, url string) *session {
	ctx, cancel := context.WithCancel(ctx)
	return &session{
		url:    url,
		ctx:    ctx,
		cancel: cancel,
		opened: make(chan error, 1),
		state:  stateReconnecting,
		stores: make(map[string]*formStore),
	}
}

// start dials, and returns once the first connection has opened or failed.
// After a failure, the session is closed.
func (s *session) start() error {
	go s.run()
	return <-s.opened
}

func (s *session) run() {
	first := true
	delay := minRedial

	for {
		err := s.connection(func() {
			delay = minRedial
			if first {
				first = false
				s.opened <- nil
			}
			s.setState(stateOpen)
		})

		if first {
			if err == nil {
				err = errClosed
			}
			s.opened <- err
			s.close()
			return
		}
		if s.ctx.Err() != nil {
			return
		}

		s.setState(stateReconnecting)
		select {
		case <-time.After(delay):
		case <-s.ctx.Done():
			return
		}
		delay = min(delay*2, maxRedial)
	}
}

// connection serves one websocket until it drops. Each connection has its
// own bindings: the server starts a new controller for it, which binds its
// forms again, and the session's stores follow them.
func (s *session) connection(connected func()) error {
	server := &jsrpc.Server{}
	browser := jsrpc.NewHeadless(server, func(name string, form *jsrpc.Form) {
		s.store(name).attach(form.Source())
	})
	defer browser.Release()

	rpcServer := rpc.NewServer()
	if err := rpcServer.Register(browser); err != nil {
		return err
	}

	defer func() {
		s.lck.Lock()
		s.server = nil
		s.lck.Unlock()
	}()

	return connection(s.ctx, s.url, server, rpcServer, func() {
		s.lck.Lock()
		s.server = server
		s.lck.Unlock()
		connected()
	})
}

func (s *session) setState(state string) {
	s.lck.Lock()
	if s.state == state || s.state == stateClosed {
		s.lck.Unlock()
		return
	}
	s.state = state
	subs := s.stateSubs.all()
	s.lck.Unlock()

	for _, cb := range subs {
		cb.Invoke(state)
	}
}

// store returns the form store for name, creating it on first use
func (s *session) store(name string) *formStore {
	s.lck.Lock()
	defer s.lck.Unlock()

	f, ok := s.stores[name]
	if !ok {
		f = newFormStore()
		s.stores[name] = f
	}
	return f
}

// close ends the session: it closes the websocket and stops redialing, and
// releases everything the page was given. The page must not use the session
// or its stores afterward.
func (s *session) close() {
	s.closed.Do(func() {
		s.cancel()
		s.setState(stateClosed)

		s.lck.Lock()
		stores := s.stores
		s.stores = make(map[string]*formStore)
		s.stateSubs.clear()
		s.lck.Unlock()

		for _, f := range stores {
			f.release()
		}
		s.funcs.Release()
	})
}

func (s *session) action(name string, args js.Value) js.Value {
	var argJSON []byte
	if !args.IsUndefined() {
		argJSON = []byte(js.Global().Get("JSON").Call("stringify", args).String())
	}

	return jsglue.PromiseResult(func() (any, error) {
		s.lck.Lock()
		server := s.server
		s.lck.Unlock()

		if server == nil {
			return failure(&rpctypes.Error{
				Code:    string(rpctypes.CodeDisconnected),
				Message: "not connected",
			}), nil
		}

		res, err := server.CallAction(name, argJSON)
		if err != nil {
			return failure(&rpctypes.Error{
				Code:    string(rpctypes.CodeDisconnected),
				Message: err.Error(),
			}), nil
		}
		if res.Error != nil {
			return failure(res.Error), nil
		}

		value := js.Undefined()
		if len(res.Value) > 0 {
			value = js.Global().Get("JSON").Call("parse", string(res.Value))
		}
		return map[string]any{"ok": true, "value": value}, nil
	})
}

func failure(e *rpctypes.Error) map[string]any {
	return map[string]any{"ok": false, "error": errorObject(e)}
}

// errorObject is an [rpctypes.Error] as the page sees it
func errorObject(e *rpctypes.Error) map[string]any {
	obj := map[string]any{
		"code":    e.Code,
		"message": e.Message,
	}
	if len(e.Params) > 0 {
		if params, ok := jstypes.ValueOf(e.Params); ok {
			obj["params"] = params
		}
	}
	return obj
}

// JsObject returns the object the page holds for the session
func (s *session) JsObject() map[string]any {
	s.lck.Lock()
	defer s.lck.Unlock()

	if s.obj != nil {
		return s.obj
	}

	s.obj = map[string]any{
		// state(): "open" | "reconnecting" | "closed"
		"state": s.funcs.FuncOf(func(js.Value, []js.Value) any {
			s.lck.Lock()
			defer s.lck.Unlock()
			return s.state
		}),

		// subscribeState(cb: (state: string) => void): () => void
		"subscribeState": s.funcs.FuncOf(func(_ js.Value, args []js.Value) any {
			if err := jsglue.AssertArgs(args, js.TypeFunction); err != nil {
				return jsglue.Error(err)
			}
			s.lck.Lock()
			id := s.stateSubs.add(args[0])
			s.lck.Unlock()

			return unsubscriber(func() {
				s.lck.Lock()
				s.stateSubs.remove(id)
				s.lck.Unlock()
			})
		}),

		// form(name: string): FormStore
		"form": s.funcs.FuncOf(func(_ js.Value, args []js.Value) any {
			if err := jsglue.AssertArgs(args, js.TypeString); err != nil {
				return jsglue.Error(err)
			}
			return s.store(args[0].String()).JsObject()
		}),

		// action(name: string, args?: any): Promise<ActionResult>
		"action": s.funcs.FuncOf(func(_ js.Value, args []js.Value) any {
			if len(args) < 1 || args[0].Type() != js.TypeString {
				return jsglue.SyncPromise(jsglue.ErrArgs)
			}
			actionArgs := js.Undefined()
			if len(args) > 1 {
				actionArgs = args[1]
			}
			return s.action(args[0].String(), actionArgs)
		}),

		// close(): void
		"close": s.funcs.FuncOf(func(js.Value, []js.Value) any {
			s.close()
			return nil
		}),
	}

	return s.obj
}

// subscribers holds callbacks by ID, so a callback can be removed even when
// the page passed the same function twice
type subscribers struct {
	next int
	cbs  map[int]js.Value
}

func (s *subscribers) add(cb js.Value) int {
	if s.cbs == nil {
		s.cbs = make(map[int]js.Value)
	}
	s.next++
	s.cbs[s.next] = cb
	return s.next
}

func (s *subscribers) remove(id int) {
	delete(s.cbs, id)
}

func (s *subscribers) clear() {
	clear(s.cbs)
}

func (s *subscribers) all() []js.Value {
	cbs := make([]js.Value, 0, len(s.cbs))
	for _, cb := range s.cbs {
		cbs = append(cbs, cb)
	}
	return cbs
}

// unsubscriber returns a function for the page to call once, which runs
// cleanup and releases itself
func unsubscriber(cleanup func()) js.Func {
	var unsubscribe js.Func
	unsubscribe = js.FuncOf(func(js.Value, []js.Value) any {
		cleanup()
		unsubscribe.Release()
		return nil
	})
	return unsubscribe
}
