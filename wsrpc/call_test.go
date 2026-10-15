package wsrpc_test

import (
	"encoding/json"
	"errors"
	"net"
	"net/rpc"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/zugcat/zugoui/controller"
	"github.com/zugcat/zugoui/wsrpc"
	"github.com/zugcat/zugoui/wsrpc/rpctypes"
)

func client(t *testing.T, s *wsrpc.Server) *rpc.Client {
	t.Helper()

	srv := rpc.NewServer()
	require.NoError(t, srv.Register(s))

	sc, cc := net.Pipe()
	go srv.ServeConn(sc)

	c := rpc.NewClient(cc)
	t.Cleanup(func() { c.Close() })
	return c
}

func call(t *testing.T, c *rpc.Client, action string, args []byte) *rpctypes.CallRes {
	t.Helper()

	res := &rpctypes.CallRes{}
	require.NoError(t, c.Call("Server.Call", &rpctypes.CallReq{Action: action, Args: args}, res))
	return res
}

func TestCall(t *testing.T) {
	s := wsrpc.New()
	c := client(t, s)

	s.HandleCall("double", func(args []byte) (any, error) {
		var n int
		if err := json.Unmarshal(args, &n); err != nil {
			return nil, err
		}
		return map[string]int{"n": n * 2}, nil
	})
	s.HandleCall("refuse", func([]byte) (any, error) {
		return nil, controller.ActionError("refused", errors.New("not now"))
	})
	s.HandleCall("panic", func([]byte) (any, error) {
		panic("oops")
	})

	res := call(t, c, "double", []byte("21"))
	assert.Nil(t, res.Error)
	assert.JSONEq(t, `{"n": 42}`, string(res.Value))

	res = call(t, c, "refuse", nil)
	assert.Equal(t, &rpctypes.Error{Code: "refused", Message: "not now"}, res.Error)

	res = call(t, c, "panic", nil)
	assert.Equal(t, string(rpctypes.CodeFailed), res.Error.Code)

	res = call(t, c, "missing", nil)
	assert.Equal(t, string(rpctypes.CodeUnknownAction), res.Error.Code)

	s.ReleaseCalls()
	res = call(t, c, "double", []byte("1"))
	assert.Equal(t, string(rpctypes.CodeUnknownAction), res.Error.Code)
}
