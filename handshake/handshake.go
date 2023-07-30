package handshake

import (
	"bytes"
	"fmt"
	"github.com/lvkeliang/P2Pin2/handshake"
	"io"
	"net"
	"time"
)

// A Handshake is a special message that a peer uses to identify itself
type Handshake struct {
	Pstr     string
	InfoHash [20]byte
	PeerID   [20]byte
}

// New creates a new handshake with the standard pstr
func New(infoHash, peerID [20]byte) *Handshake {
	return &Handshake{
		Pstr:     "BitTorrent protocol",
		InfoHash: infoHash,
		PeerID:   peerID,
	}
}

// Serialize serializes the handshake to a buffer
func (h *Handshake) Serialize() []byte {
	buf := make([]byte, len(h.Pstr)+49)
	buf[0] = byte(len(h.Pstr))
	curr := 1
	curr += copy(buf[curr:], h.Pstr)
	curr += copy(buf[curr:], make([]byte, 8)) // 8 reserved bytes
	curr += copy(buf[curr:], h.InfoHash[:])
	curr += copy(buf[curr:], h.PeerID[:])
	return buf
}

// Read parses a handshake from a stream
func Read(r io.Reader) (*Handshake, error) {
	lengthBuf := make([]byte, 1)
	_, err := io.ReadFull(r, lengthBuf)
	if err != nil {
		return nil, err
	}
	pstrlen := int(lengthBuf[0])

	if pstrlen == 0 {
		err := fmt.Errorf("pstrlen cannot be 0")
		return nil, err
	}

	handshakeBuf := make([]byte, 48+pstrlen)
	_, err = io.ReadFull(r, handshakeBuf)
	if err != nil {
		return nil, err
	}

	var infoHash, peerID [20]byte

	copy(infoHash[:], handshakeBuf[pstrlen+8:pstrlen+8+20])
	copy(peerID[:], handshakeBuf[pstrlen+8+20:])

	h := Handshake{
		Pstr:     string(handshakeBuf[0:pstrlen]),
		InfoHash: infoHash,
		PeerID:   peerID,
	}

	return &h, nil
}

func PeerHandshake(conn net.Conn, hashmap map[[20]byte]string, peerID [20]byte) (res *handshake.Handshake, filePath string, err error) {
	conn.SetDeadline(time.Now().Add(3 * time.Second))
	defer conn.SetDeadline(time.Time{}) // Disable the deadline

	res, err = handshake.Read(conn)
	if err != nil {
		return nil, "", fmt.Errorf("%v\n", err)
	}

	var infoHash [20]byte

	flag := false
	for infoHash, filePath = range hashmap {
		if bytes.Equal(res.InfoHash[:], infoHash[:]) {
			flag = true
			break
		}
	}

	if flag {
		req := handshake.New(infoHash, peerID)
		_, err := conn.Write(req.Serialize())
		if err != nil {
			return nil, "", err
		}
		return res, filePath, nil
	} else {
		return nil, "", fmt.Errorf("no file matches with infohash: %v\n", res.InfoHash)
	}

}
