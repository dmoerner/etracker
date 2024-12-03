// A tracker does not need a full bencode implementation, but only needs to encode
// error messages and peer list dicts. We therefore implement these two functions,
// rather than relying on a full library (with reflection) for bencoding.

package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"log"
)

const Interval = "1800" // 30 minutes

// FailureReason generates a bencoded failure reason from a string.
// According to BEP 3, this should be the only key included on an error.
func FailureReason(msg string) []byte {
	var bencoded bytes.Buffer
	_, err := fmt.Fprintf(&bencoded, "d14:failure reason%d:%se", len(msg), msg)
	if err != nil {
		log.Fatal(err)
	}
	return bencoded.Bytes()
}

// PeerList returns a bencoded list of peers using the compact format.
// For more information, see BEP 23.
func PeerList(peers []Peer) []byte {
	var bencodedPeers bytes.Buffer
	peersLength := 0
	for i := range peers {
		n, err := bencodedPeers.Write([]byte(peers[i].ip.To4()))
		if err != nil {
			log.Fatal(err)
		}
		// Write a two-byte array for the port.
		err = binary.Write(&bencodedPeers, binary.BigEndian, uint16(peers[i].port))
		if err != nil {
			log.Fatal(err)
		}
		peersLength += n + 2
	}
	var bencoded bytes.Buffer
	_, err := fmt.Fprintf(&bencoded, "d8:interval%d:%s5:peers%d:%se",
		len(Interval),
		Interval,
		peersLength,
		bencodedPeers.Bytes())
	if err != nil {
		log.Fatal(err)
	}
	return bencoded.Bytes()
}
