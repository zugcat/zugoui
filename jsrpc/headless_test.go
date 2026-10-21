//go:build js

package jsrpc

import (
	"encoding/gob"
	"fmt"
	"net"
	"net/rpc"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/zugcat/zugoui/observable"
	"github.com/zugcat/zugoui/observable/observabletest"
	"github.com/zugcat/zugoui/wsrpc"
)

type HeadlessAnswer struct {
	Label  string
	Picked bool
}

type HeadlessModel struct {
	Question string
	Answers  []HeadlessAnswer
}

func TestHeadless(t *testing.T) {
	gob.Register(&HeadlessModel{})
	gob.Register([]HeadlessAnswer(nil))

	s := wsrpc.New()
	s.HandleCall("echo", func(args []byte) (any, error) { return string(args), nil })

	// the last value the browser set, as an action on the server sees it
	last := &lastValue{}
	s.HandleCall("last", func([]byte) (any, error) { return last.get(), nil })

	forms := make(chan *Form, 1)

	sp_s, sp_c := net.Pipe()
	bp_s, bp_c := net.Pipe()

	srpc := rpc.NewServer()
	require.NoError(t, srpc.Register(s))
	sclient := &Server{Client: rpc.NewClient(sp_c)}

	b := NewHeadless(sclient, func(name string, f *Form) {
		assert.Equal(t, "quiz", name)
		forms <- f
	})
	brpc := rpc.NewServer()
	require.NoError(t, brpc.Register(b))
	bclient := wsrpc.Browser{Client: rpc.NewClient(bp_c)}

	wg := &sync.WaitGroup{}
	wg.Go(func() { defer sp_s.Close(); srpc.ServeConn(sp_s) })
	wg.Go(func() { defer bp_s.Close(); brpc.ServeConn(bp_s) })
	defer func() {
		sp_c.Close()
		bp_c.Close()
		wg.Wait()
		b.Release()
	}()

	m := &HeadlessModel{Question: "Which?", Answers: []HeadlessAnswer{{Label: "A"}}}
	h, err := bclient.NewValueBinding("quizmaster", "quiz", []string{"quiz"}, m)
	require.NoError(t, err)

	f := <-forms
	assert.Equal(t, "Which?", f.Source().Value("Question"))

	// a click binding or a data list binds nothing headless, and doesn't fail
	_, err = bclient.NewClickBinding("quiz.next", "next")
	require.NoError(t, err)
	require.NoError(t, bclient.AddData("quiz", []string{"select"}, 0, "zero"))

	// the browser's change reaches the server
	ob, ch := observabletest.New()
	defer close(ch)
	s.AddValueObserver("quizmaster", ob)

	require.NoError(t, observable.SetKeyPath(f.Source(), "Answers.0.Picked", true))
	got := <-ch
	assert.Equal(t, "Answers.0.Picked", got.Key)
	assert.Equal(t, true, got.Value)

	// the server's whole new slice reaches the browser
	wsrpc.Observer{Browser: bclient, Handle: h}.SetValue("Answers", []HeadlessAnswer{{Label: "B"}, {Label: "C"}})
	answers := f.Source().Value("Answers").(observable.Source)
	assert.Equal(t, []string{"0", "1"}, answers.Keys())

	// an action called straight after a set sees the value: the call waits
	// for the set to reach the server
	s.RemoveValueObservers("quizmaster")
	s.AddValueObserver("quizmaster", last)
	for n := range 20 {
		require.NoError(t, observable.SetKeyPath(f.Source(), "Question", fmt.Sprint("q", n)))
		res, err := sclient.CallAction("last", nil)
		require.NoError(t, err)
		assert.JSONEq(t, fmt.Sprintf("%q", fmt.Sprint("q", n)), string(res.Value))
	}

	// and actions answer with their results
	res, err := sclient.CallAction("echo", []byte(`"hi"`))
	require.NoError(t, err)
	assert.Nil(t, res.Error)
	assert.JSONEq(t, `"\"hi\""`, string(res.Value))
}

type lastValue struct {
	observable.NullObserver
	lck   sync.Mutex
	value any
}

func (l *lastValue) SetValue(key string, value any) {
	l.lck.Lock()
	defer l.lck.Unlock()
	l.value = value
}

func (l *lastValue) get() any {
	l.lck.Lock()
	defer l.lck.Unlock()
	return l.value
}
