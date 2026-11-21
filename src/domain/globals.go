package domain

import (
	"sync"

	"github.com/ranon-rat/kademlia-dht-implementation/src/domain/custom"
	"github.com/ranon-rat/kademlia-dht-implementation/src/domain/identity"
)

type KademliaGlobals struct {
	HashMap      *sync.Map
	RoutingTable *custom.RoutingTable
	Identity     identity.KademliaIdentity
}

func StartKademliaGlobals(ID uint64, port int) *KademliaGlobals {
	identity := identity.NewKademliaIdentity(ID, port)
	routingTable := custom.NewRoutingTable(identity, 64, 4, 3)
	hashMap := new(sync.Map)

	return &KademliaGlobals{
		HashMap:      hashMap,
		RoutingTable: routingTable,
		Identity:     identity,
	}

}
