package main

import (
	"bytes"
	"encoding/binary"
	"log"
	"math"
	"math/rand"
	"net"
	"testing"

	bencode "github.com/jackpal/bencode-go"
)

func TestFail(t *testing.T) {
	result := FailureReason("not implemented")

	var expected bytes.Buffer
	err := bencode.Marshal(&expected, map[string]string{"failure reason": "not implemented"})
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(result, expected.Bytes()) {
		t.Errorf("Expected %s, got %s\n", expected.Bytes(), result)
	}
}

// reflectExpected uses "github.com/jackpal/bencode-go" to generate reference
// expected bencode results. That is a fully-functioned library which uses
// reflection to bencode arbitrary data structures.
func reflectExpected(peers []Peer) []byte {
	var expectedPeers bytes.Buffer
	for i := range peers {
		expectedPeers.Write([]byte(peers[i].ip.To4()))
		binary.Write(&expectedPeers, binary.BigEndian, uint16(peers[i].port))
	}
	expectedMap := map[string]string{
		"interval": "1800",
		"peers":    expectedPeers.String(),
	}
	var expected bytes.Buffer
	err := bencode.Marshal(&expected, expectedMap)
	if err != nil {
		log.Fatal(err)
	}
	return expected.Bytes()
}

func TestPeers(t *testing.T) {
	peers := []Peer{
		{net.ParseIP("10.0.0.1"), 8080},
		{net.ParseIP("10.0.0.2"), 8080},
		{net.ParseIP("10.0.0.3"), 8080},
		{net.ParseIP("10.0.0.4"), 8080},
		{net.ParseIP("10.0.0.5"), 8080},
	}

	result := PeerList(peers)

	expected := reflectExpected(peers)

	if !bytes.Equal(result, expected) {
		t.Errorf("Expected %v, got %v\n", expected, result)
	}
}

// randomPeer generates random peers for benchmarking. Adapted from
// https://gist.github.com/porjo/f1e6b79af77893ee71e857dfba2f8e9a
func randomPeer() Peer {
	ipSlice := make([]byte, 4)
	binary.LittleEndian.PutUint32(ipSlice, rand.Uint32())
	ip := net.ParseIP(string(ipSlice))
	port := rand.Intn(int(math.Pow(2, 16)))
	return Peer{ip, port}
}

var blackhole []byte

// To my surprise, these benchmarks suggest that there is no significant
// difference between reflection and my implementation, even with impossibly
// large peer lists.
func BenchmarkNonReflect(b *testing.B) {
	size := 1000
	data := make([]Peer, size)
	for i := 0; i < size; i++ {
		data = append(data, randomPeer())
	}
	for i := 0; i < b.N; i++ {
		result := PeerList(data)
		blackhole = result
	}
}

func BenchmarkReflectLibrary(b *testing.B) {
	size := 1000
	data := make([]Peer, size)
	for i := 0; i < size; i++ {
		data = append(data, randomPeer())
	}
	for i := 0; i < b.N; i++ {
		result := reflectExpected(data)
		blackhole = result
	}
}
