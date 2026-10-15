//go:build js

package browser

import (
	"syscall/js"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/zugcat/zugoui/observable"
	"github.com/zugcat/zugoui/observable/controllers"
)

type testAnswer struct {
	Label  string `json:"label"`
	Picked bool   `json:"picked"`
}

type testPanel struct {
	Question string       `json:"question"`
	Answers  []testAnswer `json:"answers"`
	PickOne  int          `json:"pickOne"`
}

func (p testPanel) ValidateModel() error {
	if p.PickOne >= len(p.Answers) {
		return observable.ValidationError{"PickOne": coded{}}
	}
	return nil
}

type coded struct{}

func (coded) Error() string          { return "PickOne is out of range" }
func (coded) Code() string           { return "outOfRange" }
func (coded) Params() map[string]any { return map[string]any{"max": 1} }

func TestFormStore(t *testing.T) {
	f := newFormStore()
	obj := js.ValueOf(f.JsObject())

	assert.True(t, obj.Call("getSnapshot").IsUndefined(), "undefined until bound")
	assert.True(t, obj.Call("set", "question", "x").InstanceOf(js.Global().Get("Error")))

	calls := 0
	unsubscribe := obj.Call("subscribe", js.FuncOf(func(js.Value, []js.Value) any {
		calls++
		return nil
	}))

	panel := &testPanel{
		Question: "Which?",
		Answers:  []testAnswer{{Label: "A"}, {Label: "B"}},
		PickOne:  -1,
	}
	model := controllers.New(panel)
	f.attach(model)
	assert.Equal(t, 1, calls)

	snapshot := obj.Call("getSnapshot")
	assert.Equal(t, "Which?", snapshot.Get("question").String())
	assert.Equal(t, "B", snapshot.Get("answers").Index(1).Get("label").String())
	assert.True(t, snapshot.Equal(obj.Call("getSnapshot")), "same object until something changes")
	assert.Equal(t, 0, js.Global().Get("Object").Call("keys", obj.Call("getErrors")).Length())

	// the page sets by JSON path, and the Go model changes
	assert.True(t, obj.Call("set", "answers.1.picked", true).IsUndefined())
	assert.True(t, panel.Answers[1].Picked)
	assert.Equal(t, 2, calls)
	changed := obj.Call("getSnapshot")
	assert.False(t, snapshot.Equal(changed))
	assert.True(t, changed.Get("answers").Index(1).Get("picked").Bool())

	// numbers from the page convert to the field's type
	assert.True(t, obj.Call("set", "pickOne", 5).IsUndefined())
	assert.Equal(t, 5, panel.PickOne)
	errs := obj.Call("getErrors")
	require.Equal(t, "outOfRange", errs.Get("pickOne").Get("code").String())
	assert.Equal(t, 1, errs.Get("pickOne").Get("params").Get("max").Int())
	assert.True(t, errs.Equal(obj.Call("getErrors")), "same object until the errors change")

	// a change from the server (a whole new slice) reaches the page too
	model.SetValue("Answers", []testAnswer{{Label: "C"}})
	assert.Equal(t, "C", obj.Call("getSnapshot").Get("answers").Index(0).Get("label").String())
	assert.Equal(t, 1, obj.Call("getSnapshot").Get("answers").Length())

	// paths are JSON names, not Go names
	assert.True(t, obj.Call("set", "Question", "x").InstanceOf(js.Global().Get("Error")))
	assert.True(t, obj.Call("set", "answers.0.picked", "not a bool").IsUndefined(), "truthiness, as elsewhere")

	unsubscribe.Invoke()
	before := calls
	assert.True(t, obj.Call("set", "question", "Why?").IsUndefined())
	assert.Equal(t, before, calls)

	// a reconnect binds a new model under the same name
	f.attach(controllers.New(&testPanel{Question: "Again?", PickOne: -1}))
	assert.Equal(t, "Again?", obj.Call("getSnapshot").Get("question").String())

	f.release()
}
