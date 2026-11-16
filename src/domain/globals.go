package domain

import (
	"github.com/ranon-rat/kademlia-dht-implementation/src/domain/custom"
	"github.com/ranon-rat/kademlia-dht-implementation/src/domain/identity"
)

type KademliaGlobals struct {
	HashMap      *custom.Dictionary[string]
	RoutingTable *custom.RoutingTable
	Identity     identity.KademliaIdentity
}

func StartKademliaGlobals(ID uint64, port int) *KademliaGlobals {
	identity := identity.NewKademliaIdentity(ID, port)
	routingTable := custom.NewRoutingTable(identity, 6, 6, 4)
	hashMap := custom.NewDictionary[string](8)

	return &KademliaGlobals{
		HashMap:      hashMap,
		RoutingTable: routingTable,
		Identity:     identity,
	}

}
