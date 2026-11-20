package domain

import (
	"github.com/ranon-rat/kademlia-dht-implementation/src/domain/custom"
	"github.com/ranon-rat/kademlia-dht-implementation/src/domain/identity"
	"sync"
)

type KademliaGlobals struct {
	HashMap      *sync.Map
	RoutingTable *custom.RoutingTable
	Identity     identity.KademliaIdentity
}

func StartKademliaGlobals(ID uint64, port int) *KademliaGlobals {
	identity := identity.NewKademliaIdentity(ID, port)
	routingTable := custom.NewRoutingTable(identity, 6, 6, 4)
	hashMap := new(sync.Map)

	return &KademliaGlobals{
		HashMap:      hashMap,
		RoutingTable: routingTable,
		Identity:     identity,
	}

}
