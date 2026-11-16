package api

import "github.com/ranon-rat/kademlia-dht-implementation/src/domain/custom"

type Store struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type ResultSearch struct {
	Value      *string                 `json:"value"`
	CloseNodes *[]custom.Distance2Node `json:"node"`
}
type Message struct {
	Content string
	ID      string
}
