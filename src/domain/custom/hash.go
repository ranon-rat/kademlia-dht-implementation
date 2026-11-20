package custom

import "hash/fnv"

func HashKey(userKey string) uint64 {
	h := fnv.New64()
	h.Write([]byte(userKey))
	return h.Sum64()
}
