//go:build js

package browser

import (
	"context"
	"fmt"
	"syscall/js"

	"github.com/zugcat/zugoui/jsglue"
)

// Serve runs zugoui for pages that render their own DOM, such as React pages.
// Instead of binding elements, it hands the page an object to connect with:
//
//	connect(endpoint: string): Promise<Session>
//
// Each session is one websocket to one endpoint, so a page can hold sessions
// to several services, and close each when it is done with it. See [session]
// for what a session offers.
//
// ready names a global function, which Serve calls once with that object and
// then deletes, so nothing of zugoui's stays global.
//
// Like [Main], Serve blocks, and should be the last thing main calls. It is
// _critical_ your main package calls gob.Register() on your shared model types.
func Serve(ctx context.Context, ready string) error {
	fn := js.Global().Get(ready)
	if fn.Type() != js.TypeFunction {
		return fmt.Errorf("zugoui: %q is not a function", ready)
	}
	js.Global().Delete(ready)

	var funcs jsglue.Funcs
	defer funcs.Release()

	fn.Invoke(map[string]any{
		// connect(endpoint: string): Promise<Session>
		"connect": funcs.FuncOf(func(_ js.Value, args []js.Value) any {
			if err := jsglue.AssertArgs(args, js.TypeString); err != nil {
				return jsglue.SyncPromise(err)
			}

			u, err := endpointURL(args[0].String())
			if err != nil {
				return jsglue.SyncPromise(err)
			}

			s := newSession(ctx, u)
			return jsglue.PromiseResult(func() (any, error) {
				if err := s.start(); err != nil {
					return nil, err
				}
				return s.JsObject(), nil
			})
		}),
	})

	<-ctx.Done()
	return ctx.Err()
}
