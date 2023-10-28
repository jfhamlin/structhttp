package structhttp

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"
)

type (
	options struct {
		routeMapper RouteMapper
	}

	Route struct {
		Method string
		Path   string

		methodIndex int
	}

	RouteMapper func(methodName string) *Route

	Option func(*options)

	HTTPStatusCoder interface {
		HTTPStatusCode() int
	}

	Error struct {
		StatusCode int
		Err        error
	}

	structHandler struct {
		structValue reflect.Value
		routes      []Route
	}
)

var (
	_ http.Handler = (*structHandler)(nil)

	errorType = reflect.TypeOf((*error)(nil)).Elem()
)

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
		routeMapper: defaultRouteMapper,
	}
	for _, opt := range opts {
		opt(o)
	}

	sv := reflect.ValueOf(s)
	sh := &structHandler{
		structValue: sv,
	}

	for i := 0; i < sv.NumMethod(); i++ {
		m := sv.Type().Method(i)

		if !allowedMethod(m.Type) {
			continue
		}

		r := o.routeMapper(m.Name)
		r.methodIndex = i
		sh.routes = append(sh.routes, *r)
	}

	return sh
}

func defaultRouteMapper(methodName string) *Route {
	return &Route{
		Method: http.MethodPost,
		Path:   "/" + methodName,
	}
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
	for _, route := range sh.routes {
		fmt.Printf("checking route %s %s against %s %s\n", route.Method, route.Path, r.Method, r.URL.Path)
		if route.Method != r.Method {
			continue
		}

		if route.Path != r.URL.Path {
			continue
		}

		m := sh.structValue.Method(route.methodIndex)
		result := m.Call(nil)
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
