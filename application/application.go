package application

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"github.com/lvkeliang/P2Pin3/bitfield"
	"github.com/lvkeliang/P2Pin3/handshake"
	"github.com/lvkeliang/P2Pin3/logic"
	"net"
	"time"
)

// A Client is a TCP connection with a peer
type Client struct {
	Conn     net.Conn
	Choked   bool
	Bitfield bitfield.Bitfield
	peer     logic.Peer
	infoHash [20]byte
	peerID   [20]byte
}

func CompleteHandshake(conn net.Conn, infohash, peerID [20]byte) (*handshake.Handshake, error) {
	conn.SetDeadline(time.Now().Add(10 * time.Second))
	defer conn.SetDeadline(time.Time{}) // Disable the deadline

	req := handshake.New(infohash, peerID)
	_, err := conn.Write(req.Serialize())
	if err != nil {
		return nil, err
	}

	res, err := handshake.Read(conn)
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(res.InfoHash[:], infohash[:]) {
		return nil, fmt.Errorf("Expected infohash %x but got %x", res.InfoHash, infohash)
	}
	return res, nil
}

func recvBitfield(conn net.Conn) (bitfield.Bitfield, error) {
	conn.SetDeadline(time.Now().Add(10 * time.Second))
	defer conn.SetDeadline(time.Time{}) // Disable the deadline
	msg, err := logic.Read(conn)
	if err != nil {
		return nil, err
	}
	if msg == nil {
		err := fmt.Errorf("Expected bitfield but got %s", msg)
		return nil, err
	}
	if msg.ID != logic.MsgBitfield {
		err := fmt.Errorf("Expected bitfield but got ID %d", msg.ID)
		return nil, err
	}

	return msg.Payload, nil
}

// New connects with a peer, completes a handshake, and receives a handshake
// returns an err if any of those fail.
func New(peer logic.Peer, peerID, infoHash [20]byte) (*Client, error) {
	conn, err := net.DialTimeout("tcp", peer.String(), 15*time.Second)
	if err != nil {
		return nil, err
	}

	_, err = CompleteHandshake(conn, infoHash, peerID)
	if err != nil {
		conn.Close()
		return nil, err
	}

	conn.Read(nil)

	bf, err := recvBitfield(conn)
	if err != nil {
		conn.Close()
		return nil, err
	}

	return &Client{
		Conn:     conn,
		Choked:   true,
		Bitfield: bf,
		peer:     peer,
		infoHash: infoHash,
		peerID:   peerID,
	}, nil
}

// Read reads and consumes a message from the connection
func (c *Client) Read() (*logic.Message, error) {
	msg, err := logic.Read(c.Conn)
	return msg, err
}

// SendRequest sends a Request message to the peer
func (c *Client) SendRequest(index, begin, length int) error {
	req := logic.FormatRequest(index, begin, length)
	//fmt.Printf("application-SendRequesr-index: %v ,begin: %v ,length: %v\n", index, begin, length)
	// fmt.Printf("Serialize: %v\n", req.Serialize())
	_, err := c.Conn.Write(req.Serialize())
	return err
}

// SendInterested sends an Interested message to the peer
func (c *Client) SendInterested() error {
	msg := logic.Message{ID: logic.MsgInterested}
	_, err := c.Conn.Write(msg.Serialize())
	return err
}

// SendNotInterested sends a NotInterested message to the peer
func (c *Client) SendNotInterested() error {
	msg := logic.Message{ID: logic.MsgNotInterested}
	_, err := c.Conn.Write(msg.Serialize())
	return err
}

// SendUnchoke sends an Unchoke message to the peer
func (c *Client) SendUnchoke() error {
	msg := logic.Message{ID: logic.MsgUnchoke}
	_, err := c.Conn.Write(msg.Serialize())
	return err
}

// SendHave sends a Have message to the peer
func (c *Client) SendHave(index int) error {
	msg := logic.FormatHave(index)
	_, err := c.Conn.Write(msg.Serialize())
	return err
}

// ParseRequest parses a REQUEST message
func ParseRequest(msg *logic.Message) (index, begin, length int, err error) {
	if msg.ID != logic.MsgRequest {
		if msg.ID == logic.MsgHave {
			return
		}
		err = fmt.Errorf("Expected REQUEST (ID %d), got ID %d", logic.MsgRequest, msg.ID)
		return
	}
	if len(msg.Payload) != 12 {
		err = fmt.Errorf("Expected payload length 12, got %d", len(msg.Payload))
		return
	}
	index = int(binary.BigEndian.Uint32(msg.Payload[0:4]))
	begin = int(binary.BigEndian.Uint32(msg.Payload[4:8]))
	length = int(binary.BigEndian.Uint32(msg.Payload[8:12]))
	return
}

func SendMessage(c net.Conn, payload []byte) error {
	msg := &logic.Message{
		ID:      logic.MsgPiece,
		Payload: payload,
	}
	_, err := c.Write(msg.Serialize())
	return err
}
