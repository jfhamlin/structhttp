package structhttp

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"reflect"
)

type (
	// MatcherFunc is a function that determines whether a request
	// matches a method. It returns the non-default arguments to pass to
	// the method, a boolean indicating whether the request matches, and
	// an error if one occurred.
	MatcherFunc func(r *http.Request, methodName string, methodArgs ...reflect.Type) (arguments []any, matches bool, err error)

	structHandler struct {
		structValue reflect.Value
		methods     []reflect.Method

		matcher MatcherFunc
	}
)

var (
	_ http.Handler = (*structHandler)(nil)

	errorType = reflect.TypeOf((*error)(nil)).Elem()
	ctxType   = reflect.TypeOf((*context.Context)(nil)).Elem()
	reqType   = reflect.TypeOf((*http.Request)(nil))
)

// Handler returns an http.Handler for the given struct.
//
// The struct must be a struct or pointer to a struct. Each method on
// the struct will be mapped to a route.
//
// # Route Mapping
//
// By default, requests are mapped to methods where the HTTP method is
// POST and the path is the method name prefixed with a slash. If a
// method accepts an *http.Request or context.Context argument, the
// value is provided directly from the incoming *http.Request. At most
// one other argument may be present, and its value will be the
// request body decoded as JSON. The matching behavior can be
// customized by providing a MatcherFunc option.
//
// # Return Values
//
// The method may return any of the following:
//
// 1. Nothing
// 2. An error
// 3. A single value
// 4. A single value and an error
//
// Methods that return anything else will not be matched.
//
// # HTTP Status Codes
//
// If the method returns an error, the error's Error() method will be
// used as the response body, and the status code will be set to 500.
// If the error implements the HTTPStatusCoder interface, the status
// code will be set to the value returned by HTTPStatusCode().
func Handler(s any, opts ...Option) http.Handler {
	o := &options{
		matcher: DefaultMatcherFunc,
	}
	for _, opt := range opts {
		opt(o)
	}

	sv := reflect.ValueOf(s)
	sh := &structHandler{
		structValue: sv,
		matcher:     o.matcher,
	}

	for i := 0; i < sv.NumMethod(); i++ {
		m := sv.Type().Method(i)

		if !allowedMethod(m.Type) {
			continue
		}

		sh.methods = append(sh.methods, m)
	}

	return sh
}

func (sh *structHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	for _, method := range sh.methods {
		argTypes := make([]reflect.Type, 0, method.Type.NumIn()-1)
		for i := 1; i < method.Type.NumIn(); i++ {
			typ := method.Type.In(i)
			switch typ {
			case ctxType, reqType:
			default:
				argTypes = append(argTypes, typ)
			}
		}

		args, matches, err := sh.matcher(r, method.Name, argTypes...)
		if !matches {
			continue
		}
		if err != nil {
			writeResponse(w, []reflect.Value{reflect.ValueOf(err)})
			return
		}

		name := method.Name

		methodArgs := make([]reflect.Value, method.Type.NumIn())
		methodArgs[0] = sh.structValue
		for i := 1; i < method.Type.NumIn(); i++ {
			argType := method.Type.In(i)
			switch argType {
			case ctxType:
				methodArgs[i] = reflect.ValueOf(r.Context())
			case reqType:
				methodArgs[i] = reflect.ValueOf(r)
			default:
				if len(args) == 0 {
					panic("not enough arguments to " + name + " method")
				}
				methodArgs[i] = reflect.ValueOf(args[0])
				args = args[1:]
			}
		}
		if len(args) > 0 {
			panic("too many arguments to " + name + " method")
		}

		result := method.Func.Call(methodArgs)
		writeResponse(w, result)
		return
	}

	http.NotFound(w, r)
}

func writeResponse(w http.ResponseWriter, out []reflect.Value) {
	if len(out) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	last := out[len(out)-1]
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
		if len(out) == 1 {
			w.WriteHeader(http.StatusNoContent)
			return
		}
	}

	// special case for returning []byte
	if bytes, ok := out[0].Interface().([]byte); ok {
		_, _ = w.Write(bytes)
		return
	}

	// encode the first return value
	if err := json.NewEncoder(w).Encode(out[0].Interface()); err != nil {
		panic(err)
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
