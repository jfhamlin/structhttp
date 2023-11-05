package structhttp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

type (
	app struct {
		result any
		err    error
	}

	testArgs struct {
		ID   int
		Name string
	}

	testCase struct {
		name               string
		httpMethod         string
		path               string
		body               string
		result             any
		err                error
		expectedStatusCode int
		expectedBody       string
	}
)

func (a *app) NoResult() {}

func (a *app) OnlyError() error {
	return a.err
}

func (a *app) OnlyResult() any {
	return a.result
}

func (a *app) ErrorAndResult() (any, error) {
	return a.result, a.err
}

func (a *app) Inputs(ctx context.Context, param *testArgs) (*testArgs, error) {
	return param, a.err
}

func (a *app) Bytes() ([]byte, error) {
	return a.result.([]byte), a.err
}

func (a *app) GetThing() (any, error) {
	return a.result, a.err
}

func (a *app) TooManyArgs(foo, bar, baz int) error {
	return a.err
}

func runTests(t *testing.T, testCases []testCase, opts ...Option) {
	t.Helper()

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			handler := Handler(&app{err: tc.err, result: tc.result}, opts...)

			req := httptest.NewRequest(tc.httpMethod, tc.path, nil)
			if tc.body != "" {
				req.Body = io.NopCloser(strings.NewReader(tc.body))
			}
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			if w.Code != tc.expectedStatusCode {
				t.Errorf("expected status code %d, got %d", tc.expectedStatusCode, w.Code)
			}
			if tc.expectedStatusCode >= 400 && tc.err != nil {
				var errMap map[string]any
				if err := json.Unmarshal(w.Body.Bytes(), &errMap); err != nil {
					t.Errorf("expected error response, got %q", w.Body.String())
				}
				if errMap["error"] != tc.err.Error() {
					t.Errorf("expected error %q, got %q", tc.err.Error(), errMap["error"])
				}
			} else {
				if w.Body.String() != tc.expectedBody {
					t.Errorf("expected body %q, got %q", tc.expectedBody, w.Body.String())
				}
			}
		})
	}
}

func TestHandlerDefault(t *testing.T) {
	testCases := []testCase{
		{
			name:               "no result",
			httpMethod:         "POST",
			path:               "/NoResult",
			expectedStatusCode: 204,
			expectedBody:       "",
		},
		{
			name:               "only error, no error",
			httpMethod:         "POST",
			path:               "/OnlyError",
			expectedStatusCode: 204,
			expectedBody:       "",
		},
		{
			name:               "only error, with error",
			httpMethod:         "POST",
			path:               "/OnlyError",
			err:                errors.New("test error"),
			expectedStatusCode: 500,
		},
		{
			name:               "only error, with HTTPStatusCoder error",
			httpMethod:         "POST",
			path:               "/OnlyError",
			err:                NewError(400, errors.New("invalid request")),
			expectedStatusCode: 400,
		},
		{
			name:               "only result",
			httpMethod:         "POST",
			path:               "/OnlyResult",
			result:             map[string]string{"foo": "bar"},
			expectedStatusCode: 200,
			expectedBody:       "{\"foo\":\"bar\"}\n",
		},
		{
			name:               "error and result, no error",
			httpMethod:         "POST",
			path:               "/ErrorAndResult",
			result:             map[string]string{"foo": "bar"},
			expectedStatusCode: 200,
			expectedBody:       "{\"foo\":\"bar\"}\n",
		},
		{
			name:               "error and result, with error",
			httpMethod:         "POST",
			path:               "/ErrorAndResult",
			result:             map[string]string{"foo": "bar"},
			err:                errors.New("test error"),
			expectedStatusCode: 500,
		},
		{
			name:               "inputs, no error",
			httpMethod:         "POST",
			path:               "/Inputs",
			body:               "{\"ID\":1,\"Name\":\"foo\"}",
			expectedStatusCode: 200,
			expectedBody:       "{\"ID\":1,\"Name\":\"foo\"}\n",
		},
		{
			name:               "inputs, malformed request",
			httpMethod:         "POST",
			path:               "/Inputs",
			body:               "",
			expectedStatusCode: 400,
			expectedBody:       "{\"error\":\"failed to decode request body: EOF\"}\n",
		},
		{
			name:               "bytes, no error",
			httpMethod:         "POST",
			path:               "/Bytes",
			result:             []byte("foo"),
			expectedStatusCode: 200,
			expectedBody:       "foo",
		},
		{
			name:               "too many args, no match",
			httpMethod:         "POST",
			path:               "/TooManyArgs",
			expectedStatusCode: 404,
			expectedBody:       "404 page not found\n",
		},
	}

	runTests(t, testCases)
}

func TestHandlerCustomMatcher(t *testing.T) {
	testCases := []testCase{
		{
			name:       "GET /thing/[id]",
			httpMethod: "GET",
			path:       "/thing/1",
			result: map[string]string{
				"id": "1",
			},
			expectedStatusCode: 200,
			expectedBody:       "{\"id\":\"1\"}\n",
		},
	}

	matcherFunc := func(r *http.Request, methodName string, methodArgs ...reflect.Type) ([]any, bool, error) {
		switch {
		case strings.HasPrefix(methodName, "Get"):
			if r.Method != http.MethodGet {
				return nil, false, nil
			}

			if len(methodArgs) == 0 {
				return nil, true, nil
			}
			if len(methodArgs) > 1 {
				return nil, false, nil
			}
			re := regexp.MustCompile(fmt.Sprintf(`^\/%s\/([a-zA-Z0-9_-]+)$`, strings.ToLower(methodName[3:])))
			m := re.FindStringSubmatch(r.URL.Path)
			if len(m) != 2 {
				return nil, false, nil
			}

			return []any{m[1]}, true, nil
		}

		return DefaultMatcherFunc(r, methodName, methodArgs...)
	}

	runTests(t, testCases, WithMatcherFunc(matcherFunc))
}
