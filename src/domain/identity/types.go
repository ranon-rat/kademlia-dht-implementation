package identity

import (
	"errors"
	"fmt"
	"math/rand/v2"
	"strconv"
	"strings"
)

type KademliaIdentity struct {
	// I believe that we wouldnt be able to create a trillion randomized ids
	ID uint64 `json:"id"` // we generate a randomized ID
	//to which place do an http request
	Url string `json:"value"`
}

func NewKademliaIdentity(ID uint64, port int) KademliaIdentity {
	if ID == 0 {
		ID = rand.Uint64()
	}
	url := fmt.Sprintf("http://localhost:%d", port)
	fmt.Println("ID:", ID, "url", url)

	return KademliaIdentity{ID: ID, Url: fmt.Sprintf("http://localhost:%d", port)}
}

func ParseKademliaIdentity(identity string) (KademliaIdentity, error) {
	args := strings.Split(identity, ",")
	if len(args) != 2 {
		return KademliaIdentity{}, errors.New("bad syntax")
	}
	idstr, url := args[0], args[1]
	id, err := strconv.ParseUint(idstr, 10, 64)
	if err != nil {
		return KademliaIdentity{}, err
	}
	return KademliaIdentity{Url: url, ID: id}, nil
}

func ParseIdentities(identitiesStr string) ([]KademliaIdentity, error) {
	identities := strings.Split(identitiesStr, ";")
	output := make([]KademliaIdentity, 0, len(identities)+1)
	for _, identityStr := range identities {

		identity, err := ParseKademliaIdentity(identityStr)
		if err != nil {
			return nil, err
		}
		output = append(output, identity)
	}
	return output, nil
}
