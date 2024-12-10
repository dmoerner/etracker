// A tracker does not need a full bencode implementation, but only needs to encode
// error messages and peer list dicts. We therefore implement these two functions,
// rather than relying on a full library (with reflection) for bencoding.

package main

import (
	"bytes"
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
func PeerList(peers [][]byte) []byte {
	joinedPeers := bytes.Join(peers, []byte(""))
	var bencoded bytes.Buffer
	_, err := fmt.Fprintf(&bencoded, "d8:interval%d:%s5:peers%d:%se",
		len(Interval),
		Interval,
		len(joinedPeers),
		joinedPeers)
	if err != nil {
		log.Fatal(err)
	}
	return bencoded.Bytes()
}
