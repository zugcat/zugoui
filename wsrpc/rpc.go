package wsrpc

import (
	"encoding/json/v2"
	"errors"
	"fmt"
	"log/slog"
	"sync"

	"github.com/zugcat/zugoui/observable"
	"github.com/zugcat/zugoui/wsrpc/rpctypes"
)

var (
	ErrInvalidHandle = errors.New("invalid handle")
)

// methods called to the server from the client

// CallFunc handles an action the browser calls by name. args is the call's
// JSON arguments, empty when there are none. The result goes back to the
// browser as JSON, and an error as an [rpctypes.Error].
type CallFunc func(args []byte) (any, error)

// Server is the server side rpc service
type Server struct {
	lck             sync.RWMutex
	valueObservers  map[string]*observable.Observe
	actionObservers *observable.Observe
	calls           map[string]CallFunc
}

// New creates a new server side rpc instance
func New() *Server {
	return &Server{
		valueObservers:  make(map[string]*observable.Observe),
		actionObservers: observable.New(),
		calls:           make(map[string]CallFunc),
	}
}

// HandleCall sets the function that answers calls to an action,
// replacing any earlier one.
func (s *Server) HandleCall(action string, fn CallFunc) {
	s.lck.Lock()
	defer s.lck.Unlock()

	s.calls[action] = fn
}

// ReleaseCalls removes every call handler
func (s *Server) ReleaseCalls() {
	s.lck.Lock()
	defer s.lck.Unlock()

	clear(s.calls)
}

// Call answers an action called from the browser. Failures of the action
// itself go back in res.Error; the returned error is only for the transport.
func (s *Server) Call(req *rpctypes.CallReq, res *rpctypes.CallRes) error {
	s.lck.RLock()
	fn := s.calls[req.Action]
	s.lck.RUnlock()

	if fn == nil {
		res.Error = &rpctypes.Error{
			Code:    string(rpctypes.CodeUnknownAction),
			Message: fmt.Sprintf("unknown action %q", req.Action),
		}
		return nil
	}

	value, err := call(req.Action, fn, req.Args)
	if err == nil {
		res.Value, err = json.Marshal(value)
	}
	res.Error = rpctypes.NewError(err)

	return nil
}

// call runs fn, turning a panic into an error: net/rpc runs each call on its
// own goroutine, where a panic would end the whole process.
func call(action string, fn CallFunc, args []byte) (value any, err error) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error(fmt.Sprintf("action %s panicked: %v", action, r))
			value, err = nil, fmt.Errorf("action %s failed", action)
		}
	}()

	return fn(args)
}

// Action receives an action from click binding
func (s *Server) Action(req *rpctypes.ActionReq, _ *bool) error {
	s.actionObservers.SetValue("action", req.Action)
	return nil
}

// AddValueObserver adds a value observer for an action
func (s *Server) AddValueObserver(action string, observer observable.Observer) {
	s.lck.Lock()
	o, ok := s.valueObservers[action]
	if !ok {
		o = observable.New()
		s.valueObservers[action] = o
	}
	s.lck.Unlock()

	o.AddObserver("", observer)
}

// RemoveValueObservers removes all value observers for an action
func (s *Server) RemoveValueObservers(action string) {
	s.lck.Lock()
	defer s.lck.Unlock()

	delete(s.valueObservers, action)
}

// AddActionObserver adds an action observer
func (s *Server) AddActionObserver(observer observable.Observer) {
	s.actionObservers.AddObserver("action", observer)
}

// ReleaseActionObservers removes all action observers for an action
func (s *Server) ReleaseActionObservers() {
	s.actionObservers.Release()
}

func (s *Server) observerAt(action string) *observable.Observe {
	s.lck.RLock() // protecting the map of observers, but not the observer itself
	defer s.lck.RUnlock()

	return s.valueObservers[action]
}

// SetValue updates a bound model value
func (s *Server) SetValue(req *rpctypes.SetValueReq, _ *bool) error {
	if o := s.observerAt(req.Action); o != nil {
		o.SetValue(req.Key, req.Value)
	}

	return nil
}

// SetValueAt
func (s *Server) SetValueAt(req *rpctypes.SetValueAtReq, _ *bool) error {
	// unsupported
	return nil
}

// InsertValueAt
func (s *Server) InsertValueAt(req *rpctypes.InsertValueAtReq, _ *bool) error {
	// unsupported
	return nil
}

// RemoveValueAt
func (s *Server) RemoveValueAt(req *rpctypes.RemoveValueAtReq, _ *bool) error {
	// unsupported
	return nil
}

// SetValueFor
func (s *Server) SetValueFor(req *rpctypes.SetValueForReq, _ *bool) error {
	// unsupported
	return nil
}

// RemoveValueFor
func (s *Server) RemoveValueFor(req *rpctypes.RemoveValueForReq, _ *bool) error {
	// unsupported
	return nil
}
