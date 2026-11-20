package events

import (
	"encoding/json/v2"
	"fmt"
	"net/http"

	"github.com/ranon-rat/kademlia-dht-implementation/src/domain/identity"
)

// [GET] /info
func GetInfo(w http.ResponseWriter, r *http.Request) {
	json.MarshalWrite(w, identityDHT)
}

func Ask4Info(urlQ string) (*identity.KademliaIdentity, error) {
	resp, err := http.Get(fmt.Sprintf("%s/info", urlQ))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	out := new(identity.KademliaIdentity)
	if err := json.UnmarshalRead(resp.Body, out); err != nil {
		return nil, err
	}
	return out, nil
}

// GetPeers returns the identities currently known in this node's routing table.
func GetPeers(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.MarshalWrite(w, routingTable.GetAllNodes())
}
