package events

import (
	"bytes"
	"encoding/json/v2"
	"fmt"
	"net/http"
	"net/url"

	"github.com/ranon-rat/kademlia-dht-implementation/src/domain/api"
	"github.com/ranon-rat/kademlia-dht-implementation/src/domain/custom"
)

// [POST] /store
func PostStore(w http.ResponseWriter, r *http.Request) {
	// this would be so I can
	var store api.Store
	if err := json.UnmarshalRead(r.Body, &store); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}

	// Swap makes storing and checking the previous value one atomic operation.
	v, exists := hashMap.Swap(store.Key, store.Value)
	if exists && v == store.Value {
		return
	}

	key := custom.HashKey(store.Key)

	possibleNodes := routingTable.GetCloseNodesToKey(key)

	for _, node := range possibleNodes {
		if node.ID == identityDHT.ID {
			continue
		}
		if StoreRequest(node.Url, &store) == nil {
			continue
		}
		routingTable.DeleteNode(node.ID)
	}
}

func StoreRequest(urlq string, store *api.Store) error {
	completeRoute := fmt.Sprintf("%s/store", urlq)
	content, _ := json.Marshal(store)
	resp, err := http.Post(completeRoute, "application/json", bytes.NewBuffer(content))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("store: %s", resp.Status)
	}
	return nil
}

// [GET] /get

func GetStore(w http.ResponseWriter, r *http.Request) {
	userKey := r.URL.Query().Get("key")
	if stored, exists := hashMap.Load(userKey); exists {
		result := stored.(string)
		json.MarshalWrite(w, api.ResultSearch{
			Value: &result,
		})
		return
	}
	key := custom.HashKey(userKey)
	closeNodes := routingTable.GetCloseNodesToKey(key)
	json.MarshalWrite(w, api.ResultSearch{
		CloseNodes: &closeNodes,
	})
}

func AskForGetStore(urlq, userkey string) (*api.ResultSearch, error) {
	baseURL, _ := url.Parse(urlq)
	params := url.Values{}
	params.Add("key", userkey)
	baseURL.RawQuery = params.Encode()
	resp, err := http.Get(baseURL.String())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	out := new(api.ResultSearch)
	if err := json.UnmarshalRead(resp.Body, out); err != nil {
		return nil, err
	}
	return out, nil
}

// [GET] /get-local-search
func GetLocalSearch(w http.ResponseWriter, r *http.Request) {
	userKey := r.URL.Query().Get("key")
	if stored, exists := hashMap.Load(userKey); exists {
		result := stored.(string)
		json.MarshalWrite(w, api.ResultSearch{
			Value: &result,
		})
		return
	}
	http.Error(w, "not found", 404)
}
