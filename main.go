package main

import (
	"encoding/hex"
	"fmt"
	"hash/crc64"
	"hash/fnv"
	"math"
	"math/rand/v2"
	"os"
	"strings"
)

type BloomFilter struct {
	workload  []bool
	inserted  int
	size      int
	inputSize int
}

func (b *BloomFilter) Init(targetRate float64, inputSize int) {
	// size of filter:
	//   -\frac{n \ln{p}}{(\ln 2) ^ {2}}
	//   where:
	//     n is the input size
	//     p is the targeted fake positive rate
	b.size = -int(math.Ceil(float64(inputSize) * math.Log(targetRate) / (math.Ln2 * math.Ln2)))
	b.inputSize = inputSize
	b.workload = make([]bool, b.size)
}

// Insert inserts the given input into the filter.
// this works by mapping the input into hash values, which stand for corresponding indices
// of the filter workload.
// for example, say we have input="asbcsaihoa", whose hash values are calculated to be 1, 3, and 7,
// we then set indices[1], indices[3] and indices[7] to 1
func (b *BloomFilter) Insert(input string) error {
	// number of hash functions:
	//   \frac{m}{n} \ln 2
	//   where:
	//     m is the size of filter
	numHashFunctions := int(float64(b.size) / float64(b.inputSize) * math.Ln2)
	// here we apply double hashing instead
	// we only calculate two hash values at first, say $h_1$ and $h_2$, and we can then generate
	// k hash values by:
	// index_i = (h_1 + i * h_2) \mod m
	indices, err := generateHashVal(numHashFunctions, input, b.size)
	if err != nil {
		return err
	}

	for _, index := range indices {
		b.workload[index] = true
	}

	return nil
}

// Exists checks if given input is considered as "exist" in the filter
// may involve fake positive, according to configured fake positive rate
func (b *BloomFilter) Exists(input string) (bool, error) {
	numHashFunctions := int(float64(b.size) / float64(b.inputSize) * math.Ln2)
	indices, err := generateHashVal(numHashFunctions, input, b.size)
	if err != nil {
		return false, err
	}
	for _, index := range indices {
		if !b.workload[index] {
			return false, nil
		}
	}
	return true, nil
}

func main() {
	b := &BloomFilter{}
	m := map[string]bool{} // to check if the filter is reporting a fake positive

	logFile, err := os.Create("statistics.log")
	if err != nil {
		return
	}
	defer logFile.Close()
	// targetRate should be close to the estimated fake positive rate
	b.Init(0.1, 1e4)
	for i := 0; i < 10000; i++ {
		str := generateString(1000)
		m[str] = true
		_ = b.Insert(str)
		logFile.WriteString(fmt.Sprintf("Inserted %s\n", str))
	}
	FPCount := 0
	for i := 0; i < 100000; i++ {
		str := generateString(1000)
		if m[str] {
			continue
		}
		logFile.WriteString(fmt.Sprintf("Checked %s\n", str))
		if ok, err := b.Exists(str); err == nil && ok {
			fmt.Printf("Found fake positive: %s\n", str)
			FPCount++
		}
	}
	fmt.Println("Fake positive rate:", float64(FPCount)/float64(100000))
}

func hexToBin(hexStr string) (string, error) {
	bytes, err := hex.DecodeString(hexStr)
	if err != nil {
		return "", err
	}

	var sb strings.Builder
	for _, b := range bytes {
		sb.WriteString(fmt.Sprintf("%08b", b))
	}

	return sb.String(), nil
}

const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

func generateString(length int) string {
	b := make([]byte, length)
	for i := range b {
		// rand.IntN provides a random index within the bounds of the charset
		b[i] = charset[rand.IntN(len(charset))]
	}
	return string(b)
}

// generateHashVal generates k hash values with double hashing
// here I apply fnv and crc64 as root hash functions, which are not crypto functions
func generateHashVal(numHashFunctions int, input string, size int) ([]uint64, error) {
	// generate crc64 hash value
	table := crc64.MakeTable(crc64.ECMA)
	h1Val := crc64.Checksum([]byte(input), table)
	// generate fnv hash value
	h2 := fnv.New64a()
	_, err := h2.Write([]byte(input))
	if err != nil {
		return nil, err
	}
	h2Val := h2.Sum64()
	// double hashing
	indices := make([]uint64, numHashFunctions)
	for i := range numHashFunctions {
		indices[i] = (h1Val + (uint64(i)*h2Val)%uint64(size)) % uint64(size)
	}

	return indices, nil
}
