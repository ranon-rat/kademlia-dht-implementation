package events

import (
	"encoding/json/v2"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"sort"
	"strconv"

	"github.com/ranon-rat/kademlia-dht-implementation/src/domain/custom"
	"github.com/ranon-rat/kademlia-dht-implementation/src/domain/identity"
)

// this just returns the closest nodes to the address that was given
// [GET] /more
func GetMore(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	id, _ := strconv.Atoi(query.Get("id"))
	urlQ := query.Get("url")

	json.MarshalWrite(w, routingTable.GetCloseNodesToKey(uint64(id)))
	if _, err := Ask4Info(urlQ); err != nil {
		return
	}
	routingTable.AddNewNode(identity.KademliaIdentity{
		ID:  uint64(id),
		Url: urlQ,
	})
}

func Ask4More(urlq string) ([]custom.Distance2Node, error) {

	baseURL, err := url.Parse(urlq)
	if err != nil {
		panic(err)
	}
	params := url.Values{}
	params.Add("url", identityDHT.Url)
	params.Add("id", strconv.Itoa(int(identityDHT.ID)))

	baseURL.RawQuery = params.Encode()
	resp, err := http.Get(baseURL.String())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var out []custom.Distance2Node
	if err := json.UnmarshalRead(resp.Body, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func AskForNodeSeries(nodes []identity.KademliaIdentity) ([]identity.KademliaIdentity, []identity.KademliaIdentity) {
	var newNodes []custom.Distance2Node
	var goodNodes []identity.KademliaIdentity

	for _, n := range nodes {
		nodes, err := Ask4More(n.Url)
		if err != nil {
			fmt.Printf("Failed to connect to: %d url: %s\n", n.ID, n.Url)
		}
		goodNodes = append(goodNodes, n)
		if len(nodes) > 0 {
			newNodes = append(newNodes, nodes...)
		}
	}
	sort.Slice(newNodes, func(i, j int) bool {
		return newNodes[i].XorDistance < newNodes[j].XorDistance
	})
	makeNewNodesOutput := make([]identity.KademliaIdentity, 0, len(newNodes))
	for _, n := range newNodes {
		makeNewNodesOutput = append(makeNewNodesOutput, n.KademliaIdentity)
	}
	return makeNewNodesOutput, goodNodes
}

// you should be starting with who you want to connect with so it sets our information here
// this will return the amount of new nodes that were added
func SetupNodes(initialNodes []identity.KademliaIdentity, maxhops int) int {
	alreadyVisited := make(map[uint64]bool)
	newNodes, goodNodes := AskForNodeSeries(initialNodes)
	for _, goodNode := range goodNodes {
		alreadyVisited[goodNode.ID] = true
		routingTable.AddNewNode(goodNode)
	}
	if len(initialNodes) == 0 {
		fmt.Println("No nodes to connect to are available")
		return len(alreadyVisited)
	}
	// first i need to delete the ones that are repeated
	newNodes = slices.DeleteFunc(newNodes, func(d identity.KademliaIdentity) bool {
		v := alreadyVisited[d.ID]
		alreadyVisited[d.ID] = true
		return v
	})
	// then i get the ones that are closer to me

	for range maxhops + 1 {
		if routingTable.AmountNodes == routingTable.MaxAmountNodes {
			return len(alreadyVisited)
		}
		newNodes, goodNodes = AskForNodeSeries(newNodes)
		for _, goodNode := range goodNodes {
			alreadyVisited[goodNode.ID] = true
			routingTable.AddNewNode(goodNode)
		}
		// i delete here the ones that i have visited already
		newNodes = slices.DeleteFunc(newNodes, func(d identity.KademliaIdentity) bool {
			v := alreadyVisited[d.ID]
			alreadyVisited[d.ID] = true
			return v
		})

	}
	return len(alreadyVisited)
}
