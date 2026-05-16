package http_statuses

import (
	"net/http"
	"testing"
)

func TestCodesMatchNetHTTP(t *testing.T) {
	cases := []struct {
		name string
		got  Code
		want int
	}{
		{"400", Code400BadRequest, http.StatusBadRequest},
		{"405", Code405MethodNotAllowed, http.StatusMethodNotAllowed},
		{"409", Code409Conflict, http.StatusConflict},
		{"422", Code422UnprocessableEntity, http.StatusUnprocessableEntity},
		{"499", Code499ClientClosedRequest, 499},
		{"500", Code500InternalServerError, http.StatusInternalServerError},
		{"501", Code501NotImplemented, http.StatusNotImplemented},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if int(tc.got) != tc.want {
				t.Errorf("got %d, want %d", int(tc.got), tc.want)
			}
		})
	}
}
