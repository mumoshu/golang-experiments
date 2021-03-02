package pkg

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHttptest(t *testing.T) {
	var servers []*httptest.Server

	for i := 0; i < 10; i++ {
		srv := createServer(fmt.Sprintf("%d", i))
		defer srv.Close()

		t.Log(srv.URL)

		servers = append(servers, srv)
	}
}

func createServer(v string) *httptest.Server {
	mux := http.NewServeMux()
	mux.Handle("/", http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(200)
		writer.Write([]byte(v))
	}))

	return httptest.NewServer(mux)
}
