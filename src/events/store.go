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
		http.Error(w, err.Error(), 404)
	}

	v, _ := hashMap.Get(store.Key)
	if v == store.Value { // this just means that we are storing this already
		return
	}

	key := custom.HashKey(store.Key)

	possibleNodes := routingTable.GetCloseNodesToKey(key)

	for _, node := range possibleNodes {
		if StoreRequest(node.Url, &store) == nil {
			continue
		}
		routingTable.DeleteNode(node.ID)
	}
	hashMap.Put(store.Key, store.Value)
}

func StoreRequest(urlq string, store *api.Store) error {
	completeRoute := fmt.Sprintf("%s/store", urlq)
	content, _ := json.Marshal(store)
	resp, err := http.Post(completeRoute, "text/json", bytes.NewBuffer(content))
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

// [GET] /get

func GetStore(w http.ResponseWriter, r *http.Request) {
	userKey := r.URL.Query().Get("key")
	if result, exists := hashMap.Get(userKey); exists {
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
	out := new(api.ResultSearch)
	if err := json.UnmarshalRead(resp.Body, out); err != nil {
		return nil, err
	}
	return out, nil
}

// [GET] /get-local-search
func GetLocalSearch(w http.ResponseWriter, r *http.Request) {
	userKey := r.URL.Query().Get("key")
	if result, exists := hashMap.Get(userKey); exists {
		json.MarshalWrite(w, api.ResultSearch{
			Value: &result,
		})
		return
	}
	http.Error(w, "not found", 404)
}
