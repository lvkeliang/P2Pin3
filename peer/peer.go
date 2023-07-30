package main

import (
	"bytes"
	"crypto/rand"
	"crypto/sha1"
	"encoding/binary"
	"fmt"
	"github.com/lvkeliang/P2Pin3/bitfield"
	"github.com/lvkeliang/P2Pin3/handshake"
	"github.com/lvkeliang/P2Pin3/logic"
	"github.com/lvkeliang/P2Pin3/torrent"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"time"
)

type TorrentFile struct {
	InfoHash    [20]byte   `json:"info_hash"`
	PieceHashes [][20]byte `json:"piece_hashes"`
}

func getBitfield(conn net.Conn, pieceLength int, file os.File) (bitfield []byte, err error) {
	fileInfo, err := file.Stat()
	if err != nil {
		return nil, err
	}

	totalPieces := int(fileInfo.Size()) / pieceLength
	if int(fileInfo.Size())%pieceLength != 0 {
		totalPieces++
	}

	bitfield = make([]byte, totalPieces/8+1)
	for i := 0; i < totalPieces; i++ {
		bitfield[i/8] |= 1 << (7 - uint(i%8))
	}

	msg := make([]byte, len(bitfield)+5)
	msg[0] = byte(len(bitfield) + 1)
	msg[4] = byte(logic.MsgBitfield)
	copy(msg[5:], bitfield)

	_, err = conn.Write(msg)
	return bitfield, err
}

func Handshake(conn net.Conn, hashmap map[[20]byte]string, peerID [20]byte) (res *handshake.Handshake, filePath string, err error) {
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

// IntToBytesBigEndian int 转大端 []byte
// 此函数摘自go整型和字节数组之间的转换（https://blog.csdn.net/xuemengrui12/article/details/106056220）
func IntToBytesBigEndian(n int64, bytesLength byte) ([]byte, error) {
	switch bytesLength {
	case 1:
		tmp := int8(n)
		bytesBuffer := bytes.NewBuffer([]byte{})
		binary.Write(bytesBuffer, binary.BigEndian, &tmp)
		return bytesBuffer.Bytes(), nil
	case 2:
		tmp := int16(n)
		bytesBuffer := bytes.NewBuffer([]byte{})
		binary.Write(bytesBuffer, binary.BigEndian, &tmp)
		return bytesBuffer.Bytes(), nil
	case 3:
		tmp := int32(n)
		bytesBuffer := bytes.NewBuffer([]byte{})
		binary.Write(bytesBuffer, binary.BigEndian, &tmp)
		return bytesBuffer.Bytes()[1:], nil
	case 4:
		tmp := int32(n)
		bytesBuffer := bytes.NewBuffer([]byte{})
		binary.Write(bytesBuffer, binary.BigEndian, &tmp)
		return bytesBuffer.Bytes(), nil
	case 5:
		tmp := n
		bytesBuffer := bytes.NewBuffer([]byte{})
		binary.Write(bytesBuffer, binary.BigEndian, &tmp)
		return bytesBuffer.Bytes()[3:], nil
	case 6:
		tmp := n
		bytesBuffer := bytes.NewBuffer([]byte{})
		binary.Write(bytesBuffer, binary.BigEndian, &tmp)
		return bytesBuffer.Bytes()[2:], nil
	case 7:
		tmp := n
		bytesBuffer := bytes.NewBuffer([]byte{})
		binary.Write(bytesBuffer, binary.BigEndian, &tmp)
		return bytesBuffer.Bytes()[1:], nil
	case 8:
		tmp := n
		bytesBuffer := bytes.NewBuffer([]byte{})
		binary.Write(bytesBuffer, binary.BigEndian, &tmp)
		return bytesBuffer.Bytes(), nil
	}
	return nil, fmt.Errorf("IntToBytesBigEndian b param is invaild")
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

// SendPiece sends a PIECE message
func SendPiece(c net.Conn, msg *logic.Message, file *os.File, pieceLength int) error {
	index, begin, length, err := ParseRequest(msg)

	if err != nil {
		return err
	}
	buf := make([]byte, length+8)
	binary.BigEndian.PutUint32(buf[0:4], uint32(index))
	binary.BigEndian.PutUint32(buf[4:8], uint32(begin))
	_, err = file.ReadAt(buf[8:], int64(index*pieceLength))
	//fmt.Println("buf: ", string(buf[8:]))
	if err != nil {
		return err
	}
	SendMessage(c, buf)
	return nil
}

func handleConnection2(conn net.Conn, hashmapPath, torrentPath string) {
	defer conn.Close()

	var peerID [20]byte
	_, err := rand.Read(peerID[:])
	if err != nil {
		return
	}

	hashmap, err := torrent.ReadInfoHashFile(hashmapPath)
	if err != nil {
		log.Fatal(err)
	}

	// 处理握手
	_, filePath, err := Handshake(conn, hashmap, peerID)
	if err != nil {
		conn.Close()
		return
	}

	t, err := torrent.LoadTorrentFile(torrentPath + filepath.Base(filePath) + ".json")
	if err != nil {
		log.Fatal(err)
	}

	var fil *os.File
	fil, err = os.Open(filePath)
	if err != nil {
		log.Fatal(err)
	}

	fileInfo, err := fil.Stat()
	if err != nil {
		fmt.Errorf("err: %v\n", err)
		return
	}
	fileSize := fileInfo.Size()
	fmt.Println("fileSize: ", fileSize)

	numPieces := fileSize / int64(t.PieceLength)
	if fileSize%int64(t.PieceLength) != 0 {
		numPieces++
	}

	bitfield := bitfield.Bitfield(make([]byte, numPieces))

	buf := make([]byte, t.PieceLength)
	for i, hash := range t.PieceHashes {
		n, err := fil.Read(buf)
		if err != nil && err != io.EOF {
			fmt.Errorf("err: %v\n", err)
			return
		}
		if sha1.Sum(buf[:n]) != hash {
			fmt.Printf("piece %v hash not match\n", i)
		} else {
			bitfield.SetPiece(i)
		}
	}

	msg := make([]byte, numPieces+5)
	byteLen, err := IntToBytesBigEndian(int64(len(bitfield)+1), 4)
	copy(msg[:4], byteLen)
	msg[4] = byte(logic.MsgBitfield)
	copy(msg[5:], bitfield)

	//fmt.Printf("%b\n", msg)

	_, err = conn.Write(msg)

	// 将文件指针移动到文件开头
	_, err = fil.Seek(0, io.SeekStart)
	if err != nil {
		fmt.Println("Error seeking to start of file:", err)
		return
	}

	requests := make(chan logic.Message)

	go func() {
		for req := range requests {
			index, begin, length, _ := ParseRequest(&req)
			//file, err := os.Open(filePath)
			if err != nil {
				log.Fatal(err)
			}

			// 构造回复
			buf := make([]byte, length+8)
			binary.BigEndian.PutUint32(buf[0:4], uint32(index))
			binary.BigEndian.PutUint32(buf[4:8], uint32(begin))
			n, err := fil.ReadAt(buf[8:], int64(index*t.PieceLength+begin))
			if err != nil && err != io.EOF {
				log.Fatal(err)
			}
			msglen := uint32(n + 9)
			msgbuf := make([]byte, msglen+4)
			binary.BigEndian.PutUint32(msgbuf[0:4], msglen)
			msgbuf[4] = byte(logic.MsgPiece)
			copy(msgbuf[5:], buf[:n+8])
			// fmt.Printf("send: index %v, begin %v, length %v\n", index, begin, length)
			_, err = conn.Write(msgbuf)
			if err != nil {
				log.Fatal(err)
			}
		}
	}()

	// 处理请求
	for {
		msg, err := logic.Read(conn)
		if err != nil {
			if err == io.EOF {
				break
			}
			log.Fatal(err)
		}

		if msg == nil {
			continue
		}

		i, j, k, err := ParseRequest(msg)
		fmt.Printf("msg: index : %v, begin %v, length %v, err %v\n", i, j, k, err)
		switch msg.ID {
		case logic.MsgRequest:
			requests <- *msg
		}
	}
}

func main() {
	var torrentPath = "./have/"
	var hashmapPath = "./hashmap/hashmap.json"
	//var dataPath = "./testdata/"

	listener, err := net.Listen("tcp", "localhost:8097")
	if err != nil {
		log.Fatal(err)
	}
	defer listener.Close()

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Fatal(err)
		}
		go handleConnection2(conn, hashmapPath, torrentPath)
	}
}
