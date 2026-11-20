package main

import (
	"context"
	"encoding/json/v2"
	"fmt"
	"log"
	"math/rand/v2"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"github.com/ranon-rat/kademlia-dht-implementation/src/domain/api"
	executionerD "github.com/ranon-rat/kademlia-dht-implementation/src/domain/executioner"
	"github.com/ranon-rat/kademlia-dht-implementation/src/domain/safe"
	"github.com/ranon-rat/kademlia-dht-implementation/src/executioner"
)

var currentRunningServers = safe.NewMap[uint64, api.RunningNode]()
var globalCtx = context.Background()
var currentDir = ""

const maxNodesAmount = 5

func GetShuffledNodeList() []api.RunningNode {
	shuffled := currentRunningServers.Values()
	rand.Shuffle(2, func(i int, j int) {
		tmp := shuffled[j]
		shuffled[j] = shuffled[i]
		shuffled[i] = tmp
	})
	return shuffled[:min(maxNodesAmount, len(shuffled))]
}

func RunningProjectsIntoString(nodes []api.RunningNode) string {
	nodesStr := make([]string, len(nodes))
	for i, n := range nodes {
		nodesStr[i] = fmt.Sprintf("%d,%s", n.ID, n.URL)
	}
	return strings.Join(nodesStr, ";")
}
func GenerateCommand(rNode *api.RunningNode) string {
	output := fmt.Sprintf("./bin/node -ID %d -port %d", rNode.ID, rNode.Port)
	if currentRunningServers.Len() > 0 {
		nodes := GetShuffledNodeList()
		output += "-servers " + RunningProjectsIntoString(nodes)
	}
	return output

}
func StartServices(conf *api.StartServices) {

	for id := range currentRunningServers.Snapshot() {
		currentRunningServers.Delete(id)
		executioner.StopProcess(id)
	}

	port := conf.FromPort
	for range conf.Amount {
		url := fmt.Sprintf("http://localhost:%d", port)
		id := rand.Uint64()
		runningNode := api.RunningNode{
			ID:   id,
			Port: port,
			URL:  url,
		}
		currentRunningServers.Set(id, runningNode)
		go executioner.StartProcess(&executionerD.StartProcess{
			Dir:     currentDir,
			ID:      id,
			Command: GenerateCommand(&runningNode),
		})
		port++
	}
}

// POST /start-services
func PostServices(w http.ResponseWriter, r *http.Request) {
	var startServices api.StartServices
	if err := json.UnmarshalRead(r.Body, startServices); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	StartServices(&startServices)
	json.MarshalWrite(w, currentRunningServers.Values())
}

// DELETE /delete-service
func TurnServiceDown(w http.ResponseWriter, r *http.Request) {

	idstr := r.URL.Query().Get("id")
	id, err := strconv.ParseUint(idstr, 10, 64)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	currentRunningServers.Delete(id)
	executioner.StopProcess(id)
	w.Write([]byte("done"))
}

// DELETE /delete-services
func TurnAllServicesDown(w http.ResponseWriter, r *http.Request) {
	for id := range currentRunningServers.Snapshot() {
		currentRunningServers.Delete(id)
		executioner.StopProcess(id)
	}
	w.Write([]byte("done"))
}

// GET /all-services
func GetAllServices(w http.ResponseWriter, r *http.Request) {
	json.MarshalWrite(w, currentRunningServers.Values())
}
func RouterSetup() {

	http.HandleFunc("/start-services", PostServices)
	http.HandleFunc("/delete-service", TurnServiceDown)
	http.HandleFunc("/delete-services", TurnAllServicesDown)
	http.HandleFunc("/all-services", GetAllServices)
	http.ListenAndServe(":8080", nil)
}

func main() {
	var err error
	currentDir, err = os.Getwd()
	if err != nil {
		panic(err)
	}
	fmt.Println(currentDir)
	var stop context.CancelFunc

	globalCtx, stop = signal.NotifyContext(
		context.Background(),
		os.Interrupt,    // SIGINT (Ctrl+C)
		syscall.SIGTERM, // SIGTERM (kill)
		syscall.SIGQUIT, // SIGQUIT (Ctrl+\)
	)
	defer stop()

	executioner.Setup()
	go RouterSetup()
	<-globalCtx.Done() // ⛔ wait for systemd signal
	log.Println("Shutdown signal received")
	executioner.StoppingAll(globalCtx)
	log.Println("Cleanup finished, exiting")
}
