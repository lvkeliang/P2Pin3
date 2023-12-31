package torrent

import (
	"crypto/rand"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"github.com/lvkeliang/P2Pin3/logic"
	"github.com/lvkeliang/P2Pin3/protocol"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"time"
)

// Port 监听地址
const Port uint16 = 6881

// TorrentFile 储存解析出的信息
type TorrentFile struct {
	Announce    string     //表示 tracker 服务器的 URL
	InfoHash    [20]byte   //字段表示文件的 info 部分的 SHA-1 哈希值
	PieceHashes [][20]byte //所有数据块的 SHA-1 哈希值，它们被连接在一起形成一个字符串
	PieceLength int
	Length      int
	Name        string
}

// 解析的 info 部分
type bencodeInfo struct {
	Pieces      string `bencode:"pieces"`
	PieceLength int    `bencode:"piece length"`
	Length      int    `bencode:"length"`
	Name        string `bencode:"name"`
}

// 解析整个文件
type bencodeTorrent struct {
	Announce string      `bencode:"announce"` //表示 tracker 服务器的 URL。
	Info     bencodeInfo `bencode:"info"`     //用于存储解析出的 info 部分信息。
}

// DownloadToFile downloads a torrent and writes it to a file
func (t *TorrentFile) DownloadToFile(path string, hashmapPath string) error {
	var peerID [20]byte
	_, err := rand.Read(peerID[:])
	if err != nil {
		return err
	}
	peers, err := t.requestPeers(peerID, Port)
	if err != nil {
		return err
	}
	torrent := protocol.Torrent{
		Peers:       peers,
		PeerID:      peerID,
		InfoHash:    t.InfoHash,
		PieceHashes: t.PieceHashes,
		PieceLength: t.PieceLength,
		Length:      t.Length,
		Name:        t.Name,
	}
	buf, err := torrent.Download()
	if err != nil {
		return err
	}
	outFile, err := os.Create(path)
	if err != nil {
		return err
	}
	defer outFile.Close()
	_, err = outFile.Write(buf)
	if err != nil {
		return err
	}

	UpdateInfoHash(t.InfoHash, path, hashmapPath)
	return nil
}

type bencodeTrackerResp struct {
	Interval int    `bencode:"interval"`
	Peers    string `bencode:"peers"`
}

// TrackerResponse 用于从 tracker 服务器请求 peer 列表。它接受一个 peer ID 和一个端口号作为参数。
// 该函数首先调用 buildTrackerURL 方法构建 tracker 服务器的 URL。
// 然后，它使用 http.Client 发送 HTTP GET 请求到构建好的 URL。
// 接着，它读取响应内容，并使用 bencode.Unmarshal 函数解析响应内容。
// 最后，它调用 logic.Unmarshal 函数将解析出的 peers 字符串转换为 logic.Peer 切片，并返回该切片。
type TrackerResponse struct {
	Peers []logic.Peer `json:"peers"`
}

func (t *TorrentFile) requestPeers(peerID [20]byte, port uint16) ([]logic.Peer, error) {
	url := t.Announce
	c := &http.Client{Timeout: 15 * time.Second}
	resp, err := c.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// 从响应体中读取数据
	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	trackerResp := TrackerResponse{}
	err = json.Unmarshal(data, &trackerResp)
	if err != nil {
		return nil, err
	}

	return trackerResp.Peers, nil
}

// 保存为json
func (tf *TorrentFile) SaveTorrentFile(filePath string, filename string, hashmapPath string) error {
	// 将结构体编码为 JSON 字符串
	data, err := json.Marshal(tf)
	if err != nil {
		return err
	}

	// 将 JSON 字符串存储到文件中
	err = ioutil.WriteFile(filename, data, 0644)
	if err != nil {
		return err
	}

	err = UpdateInfoHash(tf.InfoHash, filePath, hashmapPath)
	if err != nil {
		return err
	}
	return nil
}

func LoadTorrentFile(filename string) (TorrentFile, error) {
	// 从文件中读取 JSON 数据
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return TorrentFile{}, err
	}

	// 将 JSON 数据解码为结构体
	var tf TorrentFile
	err = json.Unmarshal(data, &tf)
	if err != nil {
		return TorrentFile{}, err
	}

	return tf, nil
}

func NewTorrentFile(filename, announce string, pieceLength int) (*TorrentFile, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		return nil, err
	}
	fileSize := fileInfo.Size()

	numPieces := fileSize / int64(pieceLength)
	if fileSize%int64(pieceLength) != 0 {
		numPieces++
	}

	pieceHashes := make([][20]byte, numPieces)
	buf := make([]byte, pieceLength)
	for i := 0; i < int(numPieces); i++ {
		n, err := file.Read(buf)
		if err != nil && err != io.EOF {
			return nil, err
		}
		hash := sha1.Sum(buf[:n])
		// fmt.Printf("buf: %v, hash: %v\n", string(buf[:n]), hash)
		pieceHashes[i] = hash
	}

	infoString := "d6:lengthi" + string(fileSize) + "e4:name" + strconv.Itoa(len(fileInfo.Name())) + ":" + fileInfo.Name() + "12:piece lengthi" + strconv.Itoa(pieceLength) + "e6:pieces" + strconv.Itoa(len(pieceHashes)*20) + ":"
	for _, hash := range pieceHashes {
		infoString += string(hash[:])
	}
	infoString += "e"

	infoHash := sha1.Sum([]byte(infoString))

	torrentFile := &TorrentFile{
		Announce:    announce,
		InfoHash:    infoHash,
		PieceHashes: pieceHashes,
		PieceLength: pieceLength,
		Length:      int(fileSize),
		Name:        fileInfo.Name(),
	}

	return torrentFile, nil
}

func UpdateInfoHash(infoHash [20]byte, filePath string, hashmapPath string) error {
	infoHashMap, err := ReadInfoHashFile(hashmapPath)
	if err != nil {
		return err
	}

	infoHashMap[infoHash] = filePath

	err = writeInfoHashFile(infoHashMap, hashmapPath)
	if err != nil {
		return err
	}

	return nil
}

func ReadInfoHashFile(hashmapPath string) (map[[20]byte]string, error) {
	var tempMap map[string]string

	infoHashMap := make(map[[20]byte]string)

	data, err := ioutil.ReadFile(hashmapPath)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	if len(data) > 0 {
		err = json.Unmarshal(data, &tempMap)
		if err != nil {
			return nil, err
		}
	}

	for k, v := range tempMap {
		// 将编码后的字符串键转换回原始类型[20]byte
		var key [20]uint8
		keyBytes, err := hex.DecodeString(k)
		if err != nil {
			// 处理错误
			return nil, err
		}
		copy(key[:], keyBytes)
		infoHashMap[key] = v
	}

	return infoHashMap, nil
}

func writeInfoHashFile(infoHashMap map[[20]byte]string, hashmapPath string) error {
	tempMap := make(map[string]string)
	for k, v := range infoHashMap {
		// 因为json只能将string作为key，而不能将[20]byte作为key，所以将键转换为字符串
		keyStr := hex.EncodeToString(k[:])
		tempMap[keyStr] = v
	}
	data, err := json.Marshal(tempMap)
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(hashmapPath, data, 0644)
	if err != nil {
		return err
	}

	return nil
}
