package structhttp

import (
	"errors"
	"net/http/httptest"
	"testing"
)

type (
	app struct {
		result any
		err    error
	}
)

func TestHandlerDefault(t *testing.T) {
	testCases := []struct {
		name               string
		httpMethod         string
		path               string
		result             any
		err                error
		expectedStatusCode int
		expectedBody       string
	}{
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
			expectedBody:       "test error\n",
		},
		{
			name:               "only error, with HTTPStatusCoder error",
			httpMethod:         "POST",
			path:               "/OnlyError",
			err:                NewError(400, errors.New("invalid request")),
			expectedStatusCode: 400,
			expectedBody:       "invalid request\n",
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
			expectedBody:       "test error\n",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			handler := Handler(&app{err: tc.err, result: tc.result})

			req := httptest.NewRequest(tc.httpMethod, tc.path, nil)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			if w.Code != tc.expectedStatusCode {
				t.Errorf("expected status code %d, got %d", tc.expectedStatusCode, w.Code)
			}
			if w.Body.String() != tc.expectedBody {
				t.Errorf("expected body %q, got %q", tc.expectedBody, w.Body.String())
			}
		})
	}
}

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
