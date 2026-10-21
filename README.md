# ZuGoUI

Stop writing controllers.
Simplify full stack development against servers written in go.

ZuGoUI is a framework allowing web application elements to be bound to
server side models. Instead of writing code to retrieve from and post
to, field by field, object by object, between a REST (or any) style
endpoint and client side code, the fields are automatically bound to
models in the server, written in go.

```
// ContactForm
type ContactForm struct {
	First       string `bind:"fname"`
	Last        string `bind:"lname"`
	Email       string `bind:"email"`
	Subject     string `bind:"subject,editorTitle"` // can bind to multiple elements
	Message     string `bind:"message"`
	EmailStatus string `bind:"emailStatus>innerText"` // property binding override
	// button ID is "submit"
}
```

And then to validate:

```
func (c ContactForm) ValidateModel() error {
	var errors = make(observable.ValidationError)

	checkString(errors, "First", c.First, 128)
	checkString(errors, "Last", c.Last, 128)
	checkString(errors, "Email", c.Email, 128)
	checkString(errors, "Subject", c.Subject, 128)
	checkString(errors, "Message", c.Message, 1000)

	if _, err := mail.ParseAddress(c.Email); err != nil {
		errors["Email"] = err
	}

	if len(errors) == 0 {
		return nil
	}

	return errors
}
```

## Features

- Client and/or server side validation sharing the same model from a common go package. The UI automatically knows the definitive validation.
- Recursive models
- One:Many, Many:Many data relationships
- Ability to bind arbitrary values to arbitrary document element properties
- Action bindings. Bind an action to a button when clicked, or have the client send arbitrary actions.
- Send CustomEvent objects from the server.
- Easily turn instances of go types into js friendly versions, and back again.

## Headless pages

A page that renders its own DOM, such as a React page, can't share it with
zugoui's element bindings. For those, the wasm's main calls `browser.Serve`
instead of `browser.Main`, and the page drives zugoui itself:

```
const session = await zugoui.connect("/app/rpc")
const form = session.form("contact")    // the model the server bound as form "contact"
form.getSnapshot()                      // its values, by json name
form.set("Email", "me@example.com")     // validated in the browser, then sent on
form.getErrors()                        // { Email: { code: "email", message: "..." } }
const result = await session.action("submit")   // { ok: true, value } or { ok: false, error }
session.close()
```

The server is unchanged, apart from the actions: `Controller.HandleAction`
answers a call by name with a result, where `BindAction` only reacts to a
click. Errors with a `Code() string` method reach the page with that code, so
it can show a translated message.

## More to come
This repository will be updated with a full readme and documentation


