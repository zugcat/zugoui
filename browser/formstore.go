//go:build js

package browser

import (
	"encoding/json/v2"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"syscall/js"

	"github.com/zugcat/zugoui/jsglue"
	"github.com/zugcat/zugoui/jsonpath"
	"github.com/zugcat/zugoui/observable"
	"github.com/zugcat/zugoui/wsrpc/rpctypes"
)

var errUnbound = errors.New("zugoui: the server has not bound this form yet")

// formStore is one form's model as the page sees it, shaped for React's
// useSyncExternalStore:
//
//	getSnapshot(): T | undefined
//	subscribe(cb: () => void): () => void
//	getErrors(): Record<string, {code, message, params?}>
//	subscribeErrors(cb: () => void): () => void
//	set(path: string, value: any): undefined | Error
//
// Values and paths use the model's json names (see [jsonpath]). getSnapshot is
// undefined until the server binds the form, and both getters return the same
// object until something changes. set validates in the browser and sends the
// change to the server, which validates it again.
//
// Errors are every rule that fails, from the first render on; the page
// decides when to show them, e.g. once a field is touched or on submit.
type formStore struct {
	lck      sync.Mutex
	source   observable.MutableSource // nil until bound
	typ      reflect.Type
	watch    *observable.PathObserver
	snapshot js.Value
	stale    bool
	errors   js.Value
	lastErrs map[string]*rpctypes.Error
	subs     subscribers
	errSubs  subscribers
	funcs    jsglue.Funcs
	obj      map[string]any
}

func newFormStore() *formStore {
	return &formStore{
		snapshot: js.Undefined(),
		errors:   js.ValueOf(map[string]any{}),
		lastErrs: map[string]*rpctypes.Error{},
	}
}

// attach follows a newly bound model, such as after a reconnect
func (f *formStore) attach(source observable.MutableSource) {
	f.lck.Lock()
	if f.watch != nil {
		f.watch.Release()
	}
	f.source = source
	f.typ = source.Model().Type()
	f.watch = observable.NewPathObserver("*", source)
	f.watch.AddObserver("", observable.NewActionObserver(func(string, any) {
		f.changed()
	}))
	f.lck.Unlock()

	f.changed()
}

// changed marks the snapshot stale, works out the errors again, and tells
// subscribers. Callbacks run outside the lock, since React reads the
// snapshot from inside them.
func (f *formStore) changed() {
	f.lck.Lock()
	f.stale = true
	errorsChanged := f.updateErrors()
	subs := f.subs.all()
	var errSubs []js.Value
	if errorsChanged {
		errSubs = f.errSubs.all()
	}
	f.lck.Unlock()

	for _, cb := range subs {
		cb.Invoke()
	}
	for _, cb := range errSubs {
		cb.Invoke()
	}
}

// updateErrors validates the model, and reports whether the errors differ
// from the last ones. Call with f.lck held.
func (f *formStore) updateErrors() bool {
	errs := make(map[string]*rpctypes.Error)

	if err := observable.ValidateSource(f.source); err != nil {
		if fields, ok := errors.AsType[observable.ValidationError](err); ok {
			for keyPath, err := range fields {
				errs[jsonpath.FromKeyPath(f.typ, keyPath)] = rpctypes.NewError(err)
			}
		} else {
			errs[""] = rpctypes.NewError(err)
		}
	}

	if reflect.DeepEqual(errs, f.lastErrs) {
		return false
	}

	obj := make(map[string]any, len(errs))
	for path, e := range errs {
		obj[path] = errorObject(e)
	}
	f.errors = js.ValueOf(obj)
	f.lastErrs = errs
	return true
}

func (f *formStore) getSnapshot() js.Value {
	f.lck.Lock()
	defer f.lck.Unlock()

	if f.source != nil && f.stale {
		// the same encoding as an action's result, so a page's generated
		// types fit both
		if b, err := json.Marshal(f.source.Model().Interface()); err == nil {
			f.snapshot = js.Global().Get("JSON").Call("parse", string(b))
		}
		f.stale = false
	}
	return f.snapshot
}

// set sets a value by its JSON path. The browser's model sends the change on
// to the server, and validating it updates the errors.
func (f *formStore) set(path string, value js.Value) error {
	f.lck.Lock()
	source, typ := f.source, f.typ
	f.lck.Unlock()

	if source == nil {
		return errUnbound
	}

	keyPath, fieldType, err := jsonpath.KeyPath(typ, path)
	if err != nil {
		return err
	}

	v := reflect.New(fieldType)
	if !jsglue.Set(v.Interface(), value) {
		return fmt.Errorf("zugoui: %s can't be set to %s", path, value.Type())
	}

	return observable.SetKeyPath(source, keyPath, v.Elem().Interface())
}

func (f *formStore) release() {
	f.lck.Lock()
	defer f.lck.Unlock()

	if f.watch != nil {
		f.watch.Release()
		f.watch = nil
	}
	f.subs.clear()
	f.errSubs.clear()
	f.funcs.Release()
}

// JsObject returns the object the page holds for the store
func (f *formStore) JsObject() map[string]any {
	f.lck.Lock()
	defer f.lck.Unlock()

	if f.obj != nil {
		return f.obj
	}

	subscribe := func(subs *subscribers) js.Func {
		return f.funcs.FuncOf(func(_ js.Value, args []js.Value) any {
			if err := jsglue.AssertArgs(args, js.TypeFunction); err != nil {
				return jsglue.Error(err)
			}
			f.lck.Lock()
			id := subs.add(args[0])
			f.lck.Unlock()

			return unsubscriber(func() {
				f.lck.Lock()
				subs.remove(id)
				f.lck.Unlock()
			})
		})
	}

	f.obj = map[string]any{
		// getSnapshot(): T | undefined
		"getSnapshot": f.funcs.FuncOf(func(js.Value, []js.Value) any {
			return f.getSnapshot()
		}),

		// subscribe(cb: () => void): () => void
		"subscribe": subscribe(&f.subs),

		// getErrors(): Record<string, {code: string, message: string, params?: object}>
		"getErrors": f.funcs.FuncOf(func(js.Value, []js.Value) any {
			f.lck.Lock()
			defer f.lck.Unlock()
			return f.errors
		}),

		// subscribeErrors(cb: () => void): () => void
		"subscribeErrors": subscribe(&f.errSubs),

		// set(path: string, value: any): undefined | Error, returning an Error
		// for a path the model doesn't have or a value its field can't hold.
		// (A Go panic here would end the wasm, so it can't throw.)
		"set": f.funcs.FuncOf(func(_ js.Value, args []js.Value) any {
			if len(args) < 2 || args[0].Type() != js.TypeString {
				return jsglue.Error(jsglue.ErrArgs)
			}
			if err := f.set(args[0].String(), args[1]); err != nil {
				return jsglue.Error(err)
			}
			return js.Undefined()
		}),
	}

	return f.obj
}
