package custom

import (
	"testing"

	"github.com/ranon-rat/kademlia-dht-implementation/src/domain/identity"
)

func TestRoutingIdentityAndDeletion(t *testing.T) {
	self := identity.KademliaIdentity{ID: 1, Url: "http://localhost:9001"}
	peer := identity.KademliaIdentity{ID: 2, Url: "http://localhost:9002"}
	rt := NewRoutingTable(self, 6, 6, 4)
	if rt.AddNewNode(self) {
		t.Fatal("added own identity as peer")
	}
	rt.AddNewNode(peer)
	rt.DeleteNode(peer.ID)
	if len(rt.GetAllNodes()) != 0 || rt.AmountNodes != 0 {
		t.Fatal("deleted peer remains in routing table")
	}
	closest := rt.GetCloseNodesToKey(1)
	if len(closest) != 1 || closest[0].KademliaIdentity != self {
		t.Fatalf("incorrect own identity: %+v", closest)
	}
}
