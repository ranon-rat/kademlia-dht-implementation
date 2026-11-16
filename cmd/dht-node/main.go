package main

import (
	"flag"
	"fmt"
	"strconv"

	"github.com/ranon-rat/kademlia-dht-implementation/src/domain"
	"github.com/ranon-rat/kademlia-dht-implementation/src/domain/identity"
	"github.com/ranon-rat/kademlia-dht-implementation/src/events"
)

var port int
var possibleServers string
var ID uint64

func init() {
	flag.IntVar(&port, "port", 0, "set the flag port")
	flag.Uint64Var(&ID, "ID", 0, "this sets the node id")
	flag.StringVar(&possibleServers, "servers", "", "this is the selection of ")
	flag.Parse()
}

func main() {
	if port == 0 {
		fmt.Println("You need to specify the port")
		return
	}
	g := domain.StartKademliaGlobals(ID, port)
	events.SetGlobals(g)
	if possibleServers != "" {
		identities, err := identity.ParseIdentities(possibleServers)
		if err != nil {
			fmt.Println(err.Error())
			return
		}
		events.SetupNodes(identities, 10)
	}
	fmt.Println(g)
	events.Setup(strconv.Itoa(port))
}
