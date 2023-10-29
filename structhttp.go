package structhttp

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"reflect"
)

type (
	options struct {
		matcher MatcherFunc
	}

	// Option is an option for Handler.
	Option func(*options)

	// MatcherFunc is a function that determines whether a request
	// matches a method. It returns the non-default arguments to pass to
	// the method, and a boolean indicating whether the request matches.
	MatcherFunc func(r *http.Request, methodName string) (arguments []any, matches bool)

	// HTTPStatusCoder is an interface for errors that can return an
	// HTTP status code.
	HTTPStatusCoder interface {
		HTTPStatusCode() int
	}

	// Error is an error that can return an HTTP status code.
	Error struct {
		StatusCode int
		Err        error
	}

	structHandler struct {
		structValue reflect.Value
		methods     map[string]reflect.Value

		matcher MatcherFunc
	}
)

var (
	_ http.Handler = (*structHandler)(nil)

	errorType = reflect.TypeOf((*error)(nil)).Elem()
	ctxType   = reflect.TypeOf((*context.Context)(nil)).Elem()
	reqType   = reflect.TypeOf((*http.Request)(nil))
)

// WithMatcherFunc returns an Option that sets the MatcherFunc for
// Handler.
func WithMatcherFunc(m MatcherFunc) Option {
	return func(o *options) {
		o.matcher = m
	}
}

// Handler returns an http.Handler for the given struct.
//
// The struct must be a struct or pointer to a struct. Each method on
// the struct will be mapped to a route.
//
// # Route Mapping
//
// # Arguments
// Method arguments may be provided in the following ways:
// 1. As a query parameter
// 2. As a path parameter
// 3. As a JSON body
//
// If a value is provided in multiple ways, precedence is as indicated
// above.
//
// # Return Values
// The method may return any of the following:
// 1. Nothing
// 2. An error
// 3. A single value
// 4. A single value and an error
//
// Methods that return anything else will be omitted from the route
// table.
//
// # HTTP Status Codes
// If the method returns an error, the error's Error() method will be
// used as the response body, and the status code will be set to 500.
// If the error implements the HTTPStatusCoder interface, the status
// code will be set to the value returned by HTTPStatusCode().
func Handler(s any, opts ...Option) http.Handler {
	o := &options{
		matcher: defaultMatcher,
	}
	for _, opt := range opts {
		opt(o)
	}

	sv := reflect.ValueOf(s)
	sh := &structHandler{
		structValue: sv,
		methods:     make(map[string]reflect.Value),
		matcher:     o.matcher,
	}

	for i := 0; i < sv.NumMethod(); i++ {
		m := sv.Type().Method(i)

		if !allowedMethod(m.Type) {
			continue
		}

		sh.methods[m.Name] = sv.Method(i)
	}

	return sh
}

func defaultMatcher(r *http.Request, methodName string) ([]any, bool) {
	return nil, r.Method == "POST" && r.URL.Path == "/"+methodName
}

func allowedMethod(typ reflect.Type) bool {
	out := typ.NumOut()
	if out > 2 {
		return false
	}

	if out == 0 {
		return true
	}

	lastOut := typ.Out(out - 1)
	if out > 1 && !lastOut.Implements(errorType) {
		return false
	}

	return true
}

func (sh *structHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	for name, method := range sh.methods {
		args, matches := sh.matcher(r, name)
		if !matches {
			continue
		}

		methodArgs := make([]reflect.Value, method.Type().NumIn())
		for i := 0; i < method.Type().NumIn(); i++ {
			argType := method.Type().In(i)
			switch argType {
			case ctxType:
				methodArgs[i] = reflect.ValueOf(r.Context())
			case reqType:
				methodArgs[i] = reflect.ValueOf(r)
			default:
				if len(args) == 0 {
					panic("not enough arguments")
				}
				methodArgs[i] = reflect.ValueOf(args[0])
				args = args[1:]
			}
		}

		result := method.Call(nil)
		if len(result) == 0 {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		last := result[len(result)-1]
		if last.Type().Implements(errorType) {
			if !last.IsNil() {
				code := http.StatusInternalServerError
				var statusCoder HTTPStatusCoder
				if errors.As(last.Interface().(error), &statusCoder) {
					code = statusCoder.HTTPStatusCode()
				}
				http.Error(w, last.Interface().(error).Error(), code)

				return
			}
			if len(result) == 1 {
				w.WriteHeader(http.StatusNoContent)
				return
			}
		}

		// encode the first return value
		out := result[0].Interface()
		if err := json.NewEncoder(w).Encode(out); err != nil {
			panic(err)
		}
		return
	}

	http.NotFound(w, r)
}

////////////////////////////////////////////////////////////////////////////////
// Status code error

// NewError returns a new Error with the given status code and wrapped
// error.
func NewError(statusCode int, err error) *Error {
	return &Error{
		StatusCode: statusCode,
		Err:        err,
	}
}

func (e *Error) Error() string {
	return e.Err.Error()
}

func (e *Error) HTTPStatusCode() int {
	return e.StatusCode
}

func (e *Error) Unwrap() error {
	return e.Err
}
