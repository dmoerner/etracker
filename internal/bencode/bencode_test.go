package bencode

import (
	"bytes"
	"encoding/binary"
	"log"
	"math"
	"math/rand"
	"net"
	"strconv"
	"testing"

	bencode_go "github.com/jackpal/bencode-go"
)

func TestFail(t *testing.T) {
	result := FailureReason("not implemented")

	var expected bytes.Buffer
	err := bencode_go.Marshal(&expected, map[string]string{"failure reason": "not implemented"})
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
func reflectExpected(peers [][]byte) []byte {
	expectedMap := map[string]string{
		"interval":     "2700",
		"min interval": "30",
		"peers":        string(bytes.Join(peers, []byte(""))),
	}
	var expected bytes.Buffer
	err := bencode_go.Marshal(&expected, expectedMap)
	if err != nil {
		log.Fatal(err)
	}
	return expected.Bytes()
}

func encodeIpPort(ip string, port string) []byte {
	var peer bytes.Buffer
	_, err := peer.Write(net.ParseIP(ip).To4())
	if err != nil {
		log.Fatal(err)
	}

	portInt, err := strconv.Atoi(port)
	if err != nil {
		log.Fatal(err)
	}

	err = binary.Write(&peer, binary.BigEndian, uint16(portInt))
	if err != nil {
		log.Fatal(err)
	}

	return peer.Bytes()
}

func TestPeers(t *testing.T) {
	peers := make([][]byte, 0, 8)
	for i := 1; i <= 8; i += 1 {
		ip := "10.0.0." + strconv.Itoa(i)
		port := "808" + strconv.Itoa(i)
		peers = append(peers, encodeIpPort(ip, port))
	}

	result := PeerList(peers)

	expected := reflectExpected(peers)

	if !bytes.Equal(result, expected) {
		t.Errorf("Expected %v, got %v\n", expected, result)
	}
}

// randomPeer generates random peers for benchmarking. Adapted from
// https://gist.github.com/porjo/f1e6b79af77893ee71e857dfba2f8e9a
func randomPeer() []byte {
	var peer bytes.Buffer
	binary.Write(&peer, binary.BigEndian, rand.Uint32())
	binary.Write(&peer, binary.BigEndian, uint16(rand.Intn(int(math.Pow(2, 16)))))
	return peer.Bytes()
}

var blackhole []byte

// To my surprise, these benchmarks suggest that there is no significant
// difference between reflection and my implementation, even with impossibly
// large peer lists.
func BenchmarkNonReflect(b *testing.B) {
	size := 1000
	data := make([][]byte, 0, size)
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
	data := make([][]byte, 0, size)
	for i := 0; i < size; i++ {
		data = append(data, randomPeer())
	}
	for i := 0; i < b.N; i++ {
		result := reflectExpected(data)
		blackhole = result
	}
}
