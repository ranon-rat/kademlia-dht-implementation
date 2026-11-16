package events

import (
	"net/http"

	"github.com/ranon-rat/kademlia-dht-implementation/src/domain"
	"github.com/ranon-rat/kademlia-dht-implementation/src/domain/api"
	"github.com/ranon-rat/kademlia-dht-implementation/src/domain/custom"
	"github.com/ranon-rat/kademlia-dht-implementation/src/domain/identity"
)

var (
	routingTable   *custom.RoutingTable
	hashMap        *custom.Dictionary[string]
	identityDHT    identity.KademliaIdentity
	recentMessages = custom.NewQueue[api.Message](10)
)

func SetGlobals(kademliaGlobals *domain.KademliaGlobals) {
	routingTable = kademliaGlobals.RoutingTable
	hashMap = kademliaGlobals.HashMap
	identityDHT = kademliaGlobals.Identity
}

func Setup(port string) {

	http.HandleFunc("/info", GetInfo)
	http.HandleFunc("/more", GetMore)
	http.HandleFunc("/store", PostStore)
	// this sends a message to all the network
	http.HandleFunc("/messages", ManageMessage)
	// this gets a specific value, if not then tries to return a hop so it tries to search for something similar
	http.HandleFunc("/get", GetStore)
	// this is just to check the local search of that information not for anything else
	http.HandleFunc("/get-local-search", GetLocalSearch)
	http.ListenAndServe(":"+port, nil)

}
