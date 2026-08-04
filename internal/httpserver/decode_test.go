package httpserver

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDecodeRejectsUnknownAndTrailingJSON(t *testing.T) {
	for _, body := range []string{`{"name":"valid","unknown":true}`, `{"name":"one"}{"name":"two"}`} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest("POST", "/", strings.NewReader(body))
		var destination struct {
			Name string `json:"name"`
		}
		if decode(recorder, request, &destination) || recorder.Code != 400 {
			t.Errorf("ambiguous body accepted: %q status=%d", body, recorder.Code)
		}
	}
}
