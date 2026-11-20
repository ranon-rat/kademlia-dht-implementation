package custom

import (
	"sort"
	"sync"

	"github.com/ranon-rat/kademlia-dht-implementation/src/domain/identity"
)

type RoutingTable struct {
	Buckets         []Queue[identity.KademliaIdentity]
	CurrentIdentity identity.KademliaIdentity
	MaxPerBucket    int
	AmountNodes     int
	MaxAmountNodes  int
	rangesBuckets   []uint64
	AmountBuckets   int
	CloseReturns    int
	mu              *sync.RWMutex
}
type Distance2Node struct {
	identity.KademliaIdentity
	XorDistance uint64
}

// you should select 6 btw thats because the key is 64 bits long and the ideal amount of buckets should be log2(n) the n being the amount of bits that a key has
func NewRoutingTable(id identity.KademliaIdentity, amountBuckets, closeReturns, maxPerBucket int) *RoutingTable {
	out := new(RoutingTable)
	*out = RoutingTable{
		Buckets:         make([]Queue[identity.KademliaIdentity], 0, amountBuckets),
		CurrentIdentity: id,
		MaxPerBucket:    maxPerBucket,
		AmountNodes:     0,
		CloseReturns:    closeReturns,
		mu:              new(sync.RWMutex),
		AmountBuckets:   amountBuckets,
		MaxAmountNodes:  maxPerBucket * amountBuckets,
	}
	for range amountBuckets {
		out.Buckets = append(out.Buckets, NewQueue[identity.KademliaIdentity](maxPerBucket))
	}
	out.PopulateRanges()
	return out
}

func (rt *RoutingTable) PopulateRanges() {
	bucketslen := rt.AmountBuckets
	rangesBuckets := make([]uint64, bucketslen)
	for i := range bucketslen {
		rangesBuckets[i] = 1 << (i + 1)
	}
	rt.rangesBuckets = rangesBuckets
}

// to know the distance we need to know the specific distance that we have relative to our position
func (rt *RoutingTable) GetCloseNodesToKey(key uint64) []Distance2Node {
	rt.mu.RLock()
	defer rt.mu.RUnlock()
	distances := make([]Distance2Node, rt.AmountNodes+1)
	currentIndex := 0
	for _, bucket := range rt.Buckets {
		for _, node := range bucket.Snapshot() {
			distances[currentIndex].XorDistance = node.ID ^ key
			distances[currentIndex].ID = node.ID
			distances[currentIndex].Url = node.Url
			currentIndex++
		}
	}

	distances[len(distances)-1].XorDistance = rt.CurrentIdentity.ID ^ key
	distances[len(distances)-1].KademliaIdentity = rt.CurrentIdentity

	sort.Slice(distances, func(i, j int) bool {

		return distances[i].XorDistance < distances[j].XorDistance
	})
	return distances[:min(rt.CloseReturns, len(distances))]
}

func (rt *RoutingTable) getIndexForLocked(newID uint64) int {
	distance := rt.CurrentIdentity.ID ^ newID
	i := 0
	movingAround := (rt.AmountBuckets) / 2
	for {
		currentValue := rt.rangesBuckets[i]
		var next uint64 = 0
		if i+1 < rt.AmountBuckets {
			next = rt.rangesBuckets[i+1]
		}
		if distance >= currentValue && (distance < next || next == 0) {
			return i
		}
		if distance >= currentValue && next != 0 {
			i = min(i+movingAround, rt.AmountBuckets-1)
			movingAround = max(movingAround/2, 1)
			continue
		}
		if distance < currentValue && i > 0 {
			i = max(i-movingAround, 0)
			movingAround = max(movingAround/2, 1)
			continue
		}
		return i
	}
}

func (rt *RoutingTable) GetIndexForNode(newID uint64) int {
	rt.mu.RLock()
	defer rt.mu.RUnlock()
	return rt.getIndexForLocked(newID)
}
func (rt *RoutingTable) AddNewNode(newIdentity identity.KademliaIdentity) bool {
	if newIdentity.ID == rt.CurrentIdentity.ID {
		return false
	}
	rt.mu.Lock()
	defer rt.mu.Unlock()
	index := rt.getIndexForLocked(newIdentity.ID)
	// i just need to verify that it hasnt done anything weird
	for _, v := range rt.Buckets[index].Snapshot() {
		if v.ID != newIdentity.ID {
			continue
		}
		return false
	}
	if rt.Buckets[index].Push(newIdentity) {
		rt.AmountNodes++
	}
	return true
}

func (rt *RoutingTable) DeleteNode(nodeID uint64) {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	removed := 0

	for i := range rt.Buckets {
		rt.Buckets[i].DeleteFunc(func(v identity.KademliaIdentity) bool {
			if v.ID != nodeID {
				return false
			}
			removed++
			return true
		})
	}
	rt.AmountNodes -= removed
}

func (rt *RoutingTable) GetAllNodes() []identity.KademliaIdentity {

	rt.mu.RLock()
	defer rt.mu.RUnlock()
	output := make([]identity.KademliaIdentity, 0, rt.MaxAmountNodes)
	for _, bucket := range rt.Buckets {
		// i should do something different here
		output = append(output, bucket.Snapshot()...)
	}
	return output
}
