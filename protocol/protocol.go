package protocol

import (
	"bytes"
	"crypto/sha1"
	"fmt"
	"github.com/lvkeliang/P2Pin3/application"
	"github.com/lvkeliang/P2Pin3/logic"
	"log"
	"runtime"
	"time"
)

// MaxBlockSize is the largest number of bytes a request can ask for
const MaxBlockSize = 16384

// MaxBacklog is the number of unfulfilled requests a client can have in its pipeline
const MaxBacklog = 5

// Torrent holds data required to download a torrent from a list of peers
type Torrent struct {
	Peers       []logic.Peer
	PeerID      [20]byte
	InfoHash    [20]byte
	PieceHashes [][20]byte
	PieceLength int
	Length      int
	Name        string
}

type pieceWork struct {
	index  int
	hash   [20]byte
	length int
}

type pieceResult struct {
	index int
	buf   []byte
}

type PieceProgress struct {
	index      int
	client     *application.Client
	buf        []byte
	downloaded int
	requested  int
	backlog    int
}

func (state *PieceProgress) readMessage() error {
	msg, err := state.client.Read() // this call blocks
	if err != nil {
		return err
	}

	if msg == nil { // keep-alive
		return nil
	}

	switch msg.ID {
	case logic.MsgUnchoke:
		state.client.Choked = false
	case logic.MsgChoke:
		state.client.Choked = true
	case logic.MsgHave:
		index, err := logic.ParseHave(msg)
		if err != nil {
			return err
		}
		state.client.Bitfield.SetPiece(index)
	case logic.MsgPiece:
		n, err := logic.ParsePiece(state.index, state.buf, msg)
		if err != nil {
			return err
		}
		state.downloaded += n
		state.backlog--
	}
	return nil
}

func attemptDownloadPiece(c *application.Client, pw *pieceWork) ([]byte, error) {
	state := PieceProgress{
		index:  pw.index,
		client: c,
		buf:    make([]byte, pw.length),
	}
	// Setting a deadline helps get unresponsive peers unstuck.
	// 30 seconds is more than enough time to download a 262 KB piece
	c.Conn.SetDeadline(time.Now().Add(30 * time.Second))
	defer c.Conn.SetDeadline(time.Time{}) // Disable the deadline
	state.client.Choked = false
	for state.downloaded < pw.length {
		// If unchoked, send requests until we have enough unfulfilled requests
		if !state.client.Choked {
			for state.backlog < MaxBacklog && state.requested < pw.length {
				blockSize := MaxBlockSize
				// Last block might be shorter than the typical block
				if pw.length-state.requested < blockSize {
					blockSize = pw.length - state.requested
				}
				err := c.SendRequest(pw.index, state.requested, blockSize)

				if err != nil {
					return nil, err
				}
				state.backlog++
				state.requested += blockSize
			}
		}

		err := state.readMessage()

		if err != nil {
			return nil, err
		}
	}

	return state.buf, nil
}

func checkIntegrity(pw *pieceWork, buf []byte) error {
	hash := sha1.Sum(buf)
	if !bytes.Equal(hash[:], pw.hash[:]) {
		return fmt.Errorf("Index %d failed integrity check, want hash: %d, got: %d\n", pw.index, pw.hash[:], hash[:])
	}
	return nil
}

func (t *Torrent) startDownloadWorker(peer logic.Peer, workQueue chan *pieceWork, results chan *pieceResult) {
	c, err := application.New(peer, t.PeerID, t.InfoHash)
	if err != nil {
		log.Printf("Could not handshake with %s. Disconnecting\n", peer.IP)
		return
	}
	defer c.Conn.Close()
	c.SendUnchoke()
	c.SendInterested()
	for pw := range workQueue {
		// fmt.Println("pw: ", *pw)
		if !c.Bitfield.HasPiece(pw.index) {

			log.Printf("Completed handshake with %s\n", peer.IP)
			workQueue <- pw // Put piece back on the queue
			continue
		}
		// Download the piece
		buf, err := attemptDownloadPiece(c, pw)

		if err != nil {
			log.Printf("*pw: %v\n", *pw)
			log.Println("Exiting", err)
			workQueue <- pw // Put piece back on the queue
			return
		}
		err = checkIntegrity(pw, buf)
		if err != nil {
			log.Printf("Piece #%d failed integrity check\n", pw.index)
			fmt.Println(err)
			workQueue <- pw // Put piece back on the queue
			continue
		}
		c.SendHave(pw.index)
		results <- &pieceResult{pw.index, buf}
	}
}

func (t *Torrent) calculateBoundsForPiece(index int) (begin int, end int) {
	begin = index * t.PieceLength
	end = begin + t.PieceLength
	if end > t.Length {
		end = t.Length
	}
	return begin, end
}

func (t *Torrent) calculatePieceSize(index int) int {
	begin, end := t.calculateBoundsForPiece(index)
	return end - begin
}

// Download downloads the torrent. This stores the entire file in memory.
func (t *Torrent) Download() ([]byte, error) {
	log.Println("Starting download for", t.Name)
	// Init queues for workers to retrieve work and send results
	workQueue := make(chan *pieceWork, len(t.PieceHashes))
	results := make(chan *pieceResult)
	for index, hash := range t.PieceHashes {
		length := t.calculatePieceSize(index)
		workQueue <- &pieceWork{index, hash, length}
	}

	// Start workers
	for _, peer := range t.Peers {
		go t.startDownloadWorker(peer, workQueue, results)
	}

	// Collect results into a buffer until full
	buf := make([]byte, t.Length)
	donePieces := 0
	startTime := time.Now()
	for donePieces < len(t.PieceHashes) {
		res := <-results
		begin, end := t.calculateBoundsForPiece(res.index)
		copy(buf[begin:end], res.buf)
		donePieces++

		percent := float64(donePieces) / float64(len(t.PieceHashes)) * 100
		elapsedTime := time.Since(startTime).Seconds()
		downloadSpeed := float64(begin) / elapsedTime
		//fmt.Println("pieceNum: ", len(t.PieceHashes))
		numWorkers := runtime.NumGoroutine() - 3 // subtract 1 for main thread
		fmt.Printf("\r(%0.2f%%) 下载了第 #%d 块，来自 %d 个节点，速度: %0.2f MB/s", percent, res.index, numWorkers, downloadSpeed/1048576)
		//log.Printf("(%0.2f%%) Downloaded piece #%d from %d peers\n", percent, res.index, numWorkers)
	}

	fmt.Printf("\n")
	close(workQueue)

	return buf, nil
}
