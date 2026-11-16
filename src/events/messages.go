package events

import (
	"bytes"
	"encoding/json/v2"
	"fmt"
	"net/http"

	"github.com/ranon-rat/kademlia-dht-implementation/src/domain/api"
)

// [ANY] /message
func ManageMessage(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "POST":
		PostMessage(w, r)
	case "GET":
		PostMessage(w, r)
	default:
		fmt.Fprint(w, "what the fuck are you doing?")
	}

}

// [POST] /message
func PostMessage(w http.ResponseWriter, r *http.Request) {
	var newMsg api.Message
	if err := json.UnmarshalRead(r.Body, &newMsg); err != nil {
		http.Error(w, err.Error(), 400)
	}
	msgs := recentMessages.Snapshot()
	for _, msg := range msgs {
		if msg.ID == newMsg.ID {
			return
		}
	}
	recentMessages.Push(newMsg)

	allNodes := routingTable.GetAllNodes()
	for _, node := range allNodes {
		if AnnounceNewMessage(node.Url, newMsg) != nil {
			routingTable.DeleteNode(node.ID)
		}
	}

}

// [GET] /message

func GetMessages(w http.ResponseWriter, r *http.Request) {
	if err := json.MarshalWrite(w, recentMessages.Snapshot()); err != nil {
		http.Error(w, err.Error(), 500)
	}
}

func AnnounceNewMessage(urlq string, msg api.Message) error {
	completeURL := fmt.Sprintf("%s/message", urlq)
	content, _ := json.Marshal(msg)
	_, err := http.Post(completeURL, "text/json", bytes.NewBuffer(content))
	return err
}
