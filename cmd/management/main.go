package main

import (
	"context"
	"encoding/json/v2"
	"fmt"
	"log"
	"math/rand/v2"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/ranon-rat/kademlia-dht-implementation/src/domain/api"
	executionerD "github.com/ranon-rat/kademlia-dht-implementation/src/domain/executioner"
	"github.com/ranon-rat/kademlia-dht-implementation/src/domain/safe"
	"github.com/ranon-rat/kademlia-dht-implementation/src/executioner"
	"github.com/ranon-rat/kademlia-dht-implementation/static"
)

var currentRunningServers = safe.NewMap[uint64, api.RunningNode]()
var currentDir string

// Serialize process lifecycle operations; the map only locks individual reads/writes.
var servicesMu sync.Mutex
var nodeTransport = &http.Transport{ResponseHeaderTimeout: 10 * time.Second}

const maxNodesAmount = 20

func GetShuffledNodeList() []api.RunningNode {
	nodes := currentRunningServers.Values()
	rand.Shuffle(len(nodes), func(i, j int) { nodes[i], nodes[j] = nodes[j], nodes[i] })
	return nodes[:min(maxNodesAmount, len(nodes))]
}

func RunningProjectsIntoString(nodes []api.RunningNode) string {
	parts := make([]string, len(nodes))
	for i, n := range nodes {
		parts[i] = fmt.Sprintf("%d,%s", n.ID, n.URL)
	}
	return strings.Join(parts, ";")
}

func GenerateCommand(node *api.RunningNode) string {
	command := fmt.Sprintf("./bin/node -ID %d -port %d", node.ID, node.Port)
	if currentRunningServers.Len() > 0 {
		command += " -servers '" + RunningProjectsIntoString(GetShuffledNodeList()) + "'"
	}
	return command
}

func stopServices() {
	for id := range currentRunningServers.Snapshot() {
		executioner.StopProcess(id)
		currentRunningServers.Delete(id)
	}
}

func StartServices(conf *api.StartServices) error {
	stopServices()
	client := &http.Client{Timeout: 200 * time.Millisecond}
	for i := range conf.Amount {
		port := conf.FromPort + i
		node := api.RunningNode{ID: rand.Uint64(), Port: port, URL: fmt.Sprintf("http://localhost:%d", port)}
		// Start each process before publishing it as a bootstrap peer.
		if err := executioner.StartProcess(&executionerD.StartProcess{Dir: currentDir, ID: node.ID, Command: GenerateCommand(&node)}); err != nil {
			return err
		}
		ready := false
		deadline := time.Now().Add(3 * time.Second)
		for time.Now().Before(deadline) {
			resp, err := client.Get(node.URL + "/info")
			if err == nil {
				var actual struct {
					ID uint64 `json:"id"`
				}
				decodeErr := json.UnmarshalRead(resp.Body, &actual)
				resp.Body.Close()
				if resp.StatusCode == http.StatusOK && decodeErr == nil && actual.ID == node.ID {
					ready = true
					break
				}
			}
			time.Sleep(50 * time.Millisecond)
		}
		if !ready {
			executioner.StopProcess(node.ID)
			stopServices()
			return fmt.Errorf("could not start node on port %d; check that the port is available and run make build-node", port)
		}
		currentRunningServers.Set(node.ID, node)
	}
	return nil
}

func PostServices(w http.ResponseWriter, r *http.Request) {
	var conf api.StartServices
	if err := json.UnmarshalRead(http.MaxBytesReader(w, r.Body, 4096), &conf); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	if conf.Amount < 1 || conf.Amount > 50 || conf.FromPort < 1024 || conf.FromPort > 65536-conf.Amount || (conf.FromPort <= 8080 && conf.FromPort+conf.Amount > 8080) {
		http.Error(w, "Use 1 to 50 nodes and ports from 1024 to 65535, excluding 8080", 400)
		return
	}
	servicesMu.Lock()
	defer servicesMu.Unlock()
	if err := StartServices(&conf); err != nil {
		http.Error(w, err.Error(), 502)
		return
	}
	GetAllServices(w, r)
}

func TurnServiceDown(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(r.URL.Query().Get("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid node id", 400)
		return
	}
	servicesMu.Lock()
	defer servicesMu.Unlock()
	if _, exists := currentRunningServers.Get(id); !exists {
		http.NotFound(w, r)
		return
	}
	executioner.StopProcess(id)
	currentRunningServers.Delete(id)
	w.WriteHeader(http.StatusNoContent)
}

func TurnAllServicesDown(w http.ResponseWriter, r *http.Request) {
	servicesMu.Lock()
	defer servicesMu.Unlock()
	stopServices()
	w.WriteHeader(http.StatusNoContent)
}

func GetAllServices(w http.ResponseWriter, r *http.Request) {
	// uint64 identifiers must travel as strings to preserve their bits in JavaScript.
	type nodeView struct {
		ID   string `json:"id"`
		Port int    `json:"port"`
		URL  string `json:"url"`
	}
	nodes := make([]nodeView, 0, currentRunningServers.Len())
	for _, n := range currentRunningServers.Values() {
		nodes = append(nodes, nodeView{strconv.FormatUint(n.ID, 10), n.Port, n.URL})
	}
	w.Header().Set("Content-Type", "application/json")
	json.MarshalWrite(w, nodes)
}

func NodeRequest(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid node id", 400)
		return
	}
	node, exists := currentRunningServers.Get(id)
	if !exists {
		http.NotFound(w, r)
		return
	}
	action := r.PathValue("action")
	allowed := map[string]string{"info": "GET", "peers": "GET", "more": "GET", "store": "POST", "get": "GET", "get-local-search": "GET", "messages": "GET,POST"}
	methods, exists := allowed[action]
	if !exists {
		http.NotFound(w, r)
		return
	}
	if !strings.Contains(","+methods+",", ","+r.Method+",") {
		w.Header().Set("Allow", methods)
		http.Error(w, "method not allowed", 405)
		return
	}
	target, _ := url.Parse(node.URL)
	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.Transport = nodeTransport
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		http.Error(w, "The node did not respond: "+err.Error(), 502)
	}
	r.URL.Path = "/" + action
	proxy.ServeHTTP(w, r)
}

func Router() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /start-services", PostServices)
	mux.HandleFunc("DELETE /delete-service", TurnServiceDown)
	mux.HandleFunc("DELETE /delete-services", TurnAllServicesDown)
	mux.HandleFunc("GET /all-services", GetAllServices)
	mux.HandleFunc("/nodes/{id}/{action}", NodeRequest)
	mux.Handle("/", http.FileServer(http.FS(static.Files)))
	return mux
}

func main() {
	var err error
	currentDir, err = os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM, syscall.SIGQUIT)
	defer stop()
	executioner.Setup()
	server := &http.Server{Addr: "127.0.0.1:8080", Handler: Router(), ReadHeaderTimeout: 5 * time.Second}
	go func() {
		log.Println("Management: http://localhost:8080")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Print(err)
			stop()
		}
	}()
	<-ctx.Done()
	shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	server.Shutdown(shutdown)
	servicesMu.Lock()
	defer servicesMu.Unlock()
	stopServices()
}
