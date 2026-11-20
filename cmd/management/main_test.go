package main

import (
	"encoding/json/v2"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/ranon-rat/kademlia-dht-implementation/src/domain/api"
)

func TestStaticAndManagementValidation(t *testing.T) {
	handler := Router()
	for _, path := range []string{"/", "/app.js", "/style.css"} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest("GET", path, nil))
		if response.Code != 200 || response.Body.Len() == 0 {
			t.Fatalf("asset %s: %d", path, response.Code)
		}
	}
	for _, body := range []string{`{`, `{"amount":0,"from_port":9000}`, `{"amount":3,"from_port":8079}`} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest("POST", "/start-services", strings.NewReader(body)))
		if response.Code != 400 {
			t.Fatalf("invalid config %s: %d", body, response.Code)
		}
	}
}

func TestSelectedNodeAndExactIdentity(t *testing.T) {
	const id uint64 = 18446744073709551615
	peer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/more" || r.URL.Query().Get("id") != strconv.FormatUint(id, 10) {
			t.Errorf("unexpected upstream request: %s", r.URL)
		}
		w.Write([]byte(`[]`))
	}))
	defer peer.Close()
	currentRunningServers.Set(id, api.RunningNode{ID: id, URL: peer.URL, Port: 9001})
	defer currentRunningServers.Delete(id)
	handler := Router()
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest("GET", "/all-services", nil))
	var nodes []struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &nodes); err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 1 || nodes[0].ID != strconv.FormatUint(id, 10) {
		t.Fatalf("identity lost precision: %s", response.Body)
	}
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest("GET", "/nodes/18446744073709551615/more?id=18446744073709551615", nil))
	if response.Code != 200 || response.Body.String() != "[]" {
		t.Fatalf("proxy response: %d %s", response.Code, response.Body)
	}
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest("POST", "/nodes/18446744073709551615/info", nil))
	if response.Code != 405 {
		t.Fatalf("unexpected method status: %d", response.Code)
	}
}
