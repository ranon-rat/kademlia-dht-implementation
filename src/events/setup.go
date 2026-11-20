package events

import (
	"net/http"
	"sync"

	"github.com/ranon-rat/kademlia-dht-implementation/src/domain"
	"github.com/ranon-rat/kademlia-dht-implementation/src/domain/api"
	"github.com/ranon-rat/kademlia-dht-implementation/src/domain/custom"
	"github.com/ranon-rat/kademlia-dht-implementation/src/domain/identity"
)

var (
	routingTable   *custom.RoutingTable
	hashMap        *sync.Map
	identityDHT    identity.KademliaIdentity
	recentMessages = custom.NewQueue[api.Message](10)
)

func SetGlobals(kademliaGlobals *domain.KademliaGlobals) {
	routingTable = kademliaGlobals.RoutingTable
	hashMap = kademliaGlobals.HashMap
	identityDHT = kademliaGlobals.Identity
}

func Router() *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /info", GetInfo)
	mux.HandleFunc("GET /more", GetMore)
	mux.HandleFunc("POST /store", PostStore)
	// this sends a message to all the network
	mux.HandleFunc("/messages", ManageMessage)
	// this gets a specific value, if not then tries to return a hop so it tries to search for something similar
	mux.HandleFunc("GET /get", GetStore)
	// this is just to check the local search of that information not for anything else
	mux.HandleFunc("GET /get-local-search", GetLocalSearch)
	return mux

}

func Setup(port string) error {
	return http.ListenAndServe(":"+port, Router())
}
