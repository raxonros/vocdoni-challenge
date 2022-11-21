package pow

import (
	"github.com/bwesterb/go-pow"
	"math/rand" 
)

func CalculatePow() bool {

	nonce := make([]byte, 256)
	rand.Read(nonce)
	req := pow.NewRequest(20, nonce)
	proof, _ := pow.Fulfil(req, []byte("data"))
	ok, _ := pow.Check(req, proof, []byte("data"))

	if ok {
		return true
	}

	return false
}