package structhttp

import (
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
)

type (
	options struct {
		matcher MatcherFunc
	}

	// Option is an option for Handler.
	Option func(*options)
)

// WithMatcherFunc returns an Option that sets the MatcherFunc for
// Handler.
func WithMatcherFunc(m MatcherFunc) Option {
	return func(o *options) {
		o.matcher = m
	}
}

// DefaultMatcherFunc is the default MatcherFunc for Handler.
func DefaultMatcherFunc(r *http.Request, methodName string, methodArgs ...reflect.Type) ([]any, bool, error) {
	if r.Method != "POST" || (r.URL.Path != "/"+methodName && r.URL.Path != methodName) {
		return nil, false, nil
	}

	if len(methodArgs) == 0 {
		return nil, true, nil
	}

	if len(methodArgs) > 1 {
		return nil, false, nil
	}

	argType := methodArgs[0]
	arg := reflect.New(argType)
	if err := json.NewDecoder(r.Body).Decode(arg.Interface()); err != nil {
		return nil, true, NewError(http.StatusBadRequest, fmt.Errorf("failed to decode request body: %w", err))
	}
	return []any{arg.Elem().Interface()}, true, nil
}
