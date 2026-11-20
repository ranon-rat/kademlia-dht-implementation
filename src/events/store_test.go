package events

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/ranon-rat/kademlia-dht-implementation/src/domain"
	"github.com/ranon-rat/kademlia-dht-implementation/src/domain/identity"
)

func TestConcurrentDuplicateStorePropagatesOnce(t *testing.T) {
	SetGlobals(domain.StartKademliaGlobals(1, 9001))
	var forwards atomic.Int32
	peer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		forwards.Add(1)
	}))
	defer peer.Close()
	routingTable.AddNewNode(identity.KademliaIdentity{ID: 2, Url: peer.URL})
	var wg sync.WaitGroup
	for range 20 {
		wg.Go(func() {
			w := httptest.NewRecorder()
			PostStore(w, httptest.NewRequest("POST", "/store", strings.NewReader(`{"key":"key","value":""}`)))
			if w.Code != 200 {
				t.Errorf("store: %d", w.Code)
			}
		})
	}
	wg.Wait()
	if forwards.Load() != 1 {
		t.Fatalf("forwarded %d times", forwards.Load())
	}
	if value, ok := hashMap.Load("key"); !ok || value != "" {
		t.Fatalf("empty value missing: %v, %v", value, ok)
	}
	// A new value for the same key must replace the old one and propagate again.
	PostStore(httptest.NewRecorder(), httptest.NewRequest("POST", "/store", strings.NewReader(`{"key":"key","value":"updated"}`)))
	if value, _ := hashMap.Load("key"); value != "updated" || forwards.Load() != 2 {
		t.Fatalf("update missing: %v, forwards=%d", value, forwards.Load())
	}
}

func TestGetKeepsReturningCandidatesAndLocalSearchStaysLocal(t *testing.T) {
	SetGlobals(domain.StartKademliaGlobals(1, 9001))
	routingTable.AddNewNode(identity.KademliaIdentity{ID: 2, Url: "http://localhost:9002"})
	router := Router()
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest("GET", "/get?key=missing", nil))
	if w.Code != 200 || !strings.Contains(w.Body.String(), "http://localhost:9002") {
		t.Fatalf("missing candidates: %d %s", w.Code, w.Body)
	}
	w = httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest("GET", "/get-local-search?key=missing", nil))
	if w.Code != 404 {
		t.Fatalf("local lookup: %d", w.Code)
	}
}
