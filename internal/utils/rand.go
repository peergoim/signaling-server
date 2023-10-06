package utils

import (
	"crypto/rand"
	"log"
	"math/big"
	rand2 "math/rand"
)

// RealRandInt returns a real-random integer between min and max
// [min, max) (including min, excluding max)
func RealRandInt(min int, max int) int {
	if max-min <= 0 {
		panic("max must be greater than min")
	}
	n, err := rand.Int(rand.Reader, big.NewInt(int64(max-min)))
	if err != nil {
		log.Printf("RealRandInt error: %v", err)
		return FakeRandInt(min, max)
	}
	return int(n.Int64()) + min
}

func FakeRandInt(min int, max int) int {
	return rand2.Intn(max-min) + min
}
