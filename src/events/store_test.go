package events

import (
	"encoding/json/v2"
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

func TestPeersReturnsKnownIdentities(t *testing.T) {
	SetGlobals(domain.StartKademliaGlobals(1, 9001))
	peer := identity.KademliaIdentity{ID: 18446744073709551615, Url: "http://localhost:9002"}
	routingTable.AddNewNode(peer)
	w := httptest.NewRecorder()
	Router().ServeHTTP(w, httptest.NewRequest("GET", "/peers", nil))
	if w.Code != 200 || w.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("peers response: %d %s", w.Code, w.Body)
	}
	var peers []identity.KademliaIdentity
	if err := json.Unmarshal(w.Body.Bytes(), &peers); err != nil {
		t.Fatal(err)
	}
	if len(peers) != 1 || peers[0] != peer {
		t.Fatalf("unexpected peers: %+v", peers)
	}
	routingTable.DeleteNode(peer.ID)
	w = httptest.NewRecorder()
	Router().ServeHTTP(w, httptest.NewRequest("GET", "/peers", nil))
	if w.Body.String() != "[]" {
		t.Fatalf("deleted peer remains: %s", w.Body)
	}
}

func TestPeersReturnsAllBuckets(t *testing.T) {
	SetGlobals(domain.StartKademliaGlobals(123, 9001))
	for bucket := range 6 {
		for offset := range 4 {
			distance := uint64(offset + 1)
			if bucket > 0 {
				distance = uint64(1)<<(58+bucket) + uint64(offset)
			}
			routingTable.AddNewNode(identity.KademliaIdentity{ID: identityDHT.ID ^ distance, Url: "http://localhost"})
		}
	}
	w := httptest.NewRecorder()
	Router().ServeHTTP(w, httptest.NewRequest("GET", "/peers", nil))
	var peers []identity.KademliaIdentity
	if err := json.Unmarshal(w.Body.Bytes(), &peers); err != nil {
		t.Fatal(err)
	}
	if len(peers) != 24 {
		t.Fatalf("/peers returned %d entries, want 24", len(peers))
	}
}
