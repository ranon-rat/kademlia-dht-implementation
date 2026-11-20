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
	Content string `json:"content"`
	ID      string `json:"id"`
}

type StartServices struct {
	FromPort int `json:"from_port"`
	Amount   int `json:"amount"`
}
type RunningNode struct {
	Port int    `json:"port"`
	ID   uint64 `json:"id"`
	URL  string `json:"url"`
}
