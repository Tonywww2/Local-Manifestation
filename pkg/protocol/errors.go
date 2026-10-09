package protocol

import "errors"

// Rejection contains a language-neutral user-facing error and named parameters.
type Rejection struct {
	Code   string
	Params map[string]string
}

func (e *Rejection) Error() string { return e.Code }

func Reject(code string, params ...string) error {
	e := &Rejection{Code: code}
	if len(params)%2 != 0 {
		panic("protocol.Reject needs parameter pairs")
	}
	if len(params) > 0 {
		e.Params = map[string]string{}
		for i := 0; i < len(params); i += 2 {
			e.Params[params[i]] = params[i+1]
		}
	}
	return e
}

func ErrorMessage(err error) Message {
	var rejection *Rejection
	if errors.As(err, &rejection) {
		return Message{Kind: "error", Code: rejection.Code, Params: rejection.Params, Text: err.Error()}
	}
	return Message{Kind: "error", Code: "error.detail", Params: map[string]string{"detail": err.Error()}, Text: err.Error()}
}
