package utils

import "testing"

func TestFakeRandInt(t *testing.T) {
	numTimes := make(map[int]int)
	for i := 0; i < 1000000; i++ {
		randInt := FakeRandInt(100, 110)
		if _, ok := numTimes[randInt]; !ok {
			numTimes[randInt] = 0
		}
		numTimes[randInt]++
	}
	t.Log(numTimes)
}
