package rpctypes

// Code names a failure zugoui reports itself, rather than one of the
// application's own. The website generates its TypeScript from this file.
type Code string

const (
	CodeFailed        Code = "failed"        // the error carried no code of its own
	CodeInvalid       Code = "invalid"       // a form fails validation; each field says why
	CodeUnknownAction Code = "unknownAction" // nothing handles the called action
	CodeDisconnected  Code = "disconnected"  // the call never got an answer
)
