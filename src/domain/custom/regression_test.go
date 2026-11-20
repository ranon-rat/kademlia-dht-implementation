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

func TestRoutingBucketBoundariesCoverUint64(t *testing.T) {
	self := identity.KademliaIdentity{ID: 123}
	rt := NewRoutingTable(self, 6, 6, 4)
	cases := []struct {
		distance uint64
		bucket   int
	}{
		{0, 0}, {1, 0}, {(uint64(1) << 59) - 1, 0},
		{uint64(1) << 59, 1}, {(uint64(1) << 60) - 1, 1},
		{uint64(1) << 60, 2}, {uint64(1) << 61, 3},
		{uint64(1) << 62, 4}, {uint64(1) << 63, 5}, {^uint64(0), 5},
	}
	for _, test := range cases {
		if got := rt.GetIndexForNode(self.ID ^ test.distance); got != test.bucket {
			t.Errorf("distance %d: bucket %d, want %d", test.distance, got, test.bucket)
		}
	}
}

func TestRoutingTableRetainsFourPeersInEachOfSixBuckets(t *testing.T) {
	self := identity.KademliaIdentity{ID: 123}
	rt := NewRoutingTable(self, 6, 6, 4)
	for bucket := range 6 {
		for offset := range 4 {
			distance := uint64(offset + 1)
			if bucket > 0 {
				distance = (uint64(1) << (58 + bucket)) + uint64(offset)
			}
			rt.AddNewNode(identity.KademliaIdentity{ID: self.ID ^ distance, Url: "http://localhost"})
		}
	}
	if got := len(rt.GetAllNodes()); got != 24 || rt.AmountNodes != 24 {
		t.Fatalf("retained %d peers, amount=%d; want 24", got, rt.AmountNodes)
	}
	for i := range rt.Buckets {
		if got := len(rt.Buckets[i].Snapshot()); got != 4 {
			t.Errorf("bucket %d has %d peers", i, got)
		}
	}
	if got := len(rt.GetCloseNodesToKey(self.ID)); got != 6 {
		t.Fatalf("closest result count: %d", got)
	}
	rt.AddNewNode(identity.KademliaIdentity{ID: self.ID ^ (uint64(1)<<63 | 100), Url: "http://localhost"})
	if got := len(rt.GetAllNodes()); got != 24 {
		t.Fatalf("overflow changed capacity: %d", got)
	}
}
