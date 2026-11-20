package events

import (
	"bytes"
	"encoding/json/v2"
	"fmt"
	"net/http"
	"sync"

	"github.com/ranon-rat/kademlia-dht-implementation/src/domain/api"
)

// [ANY] /message
var messageMu sync.Mutex

func ManageMessage(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "POST":
		PostMessage(w, r)
	case "GET":
		GetMessages(w, r)
	default:
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}

}

// [POST] /message
func PostMessage(w http.ResponseWriter, r *http.Request) {
	var newMsg api.Message
	if err := json.UnmarshalRead(r.Body, &newMsg); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	messageMu.Lock()
	msgs := recentMessages.Snapshot()
	for _, msg := range msgs {
		if msg.ID == newMsg.ID {
			messageMu.Unlock()
			return
		}
	}
	recentMessages.Push(newMsg)
	messageMu.Unlock()

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
	completeURL := fmt.Sprintf("%s/messages", urlq)
	content, _ := json.Marshal(msg)
	resp, err := http.Post(completeURL, "text/json", bytes.NewBuffer(content))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("message: %s", resp.Status)
	}
	return nil
}
