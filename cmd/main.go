package main

import (
	"fmt"
	"github.com/lvkeliang/P2Pin3/torrent"
	"log"
	"os"
)

func main() {
	//inPath := "F:\\torrent\\[ANi] 殭屍 100～在成為殭屍前要做的 100 件事～ - 01 [1080P][Baha][WEB-DL][AAC AVC][CHT].mp4.torrent"
	outPath := "./downloaded/"

	//newtorrent, err := torrent.NewTorrentFile("./testdata/97806585_p0.jpg", "http://localhost:8090/announce", 12*1024)
	newtorrent, err := torrent.NewTorrentFile("./testdata/97806585_p0.jpg", "http://localhost:8090/announce", 12*1024)
	if err != nil {
		log.Fatal(err)
	}
	err = newtorrent.SaveTorrentFile("./have/result.json")

	t, err := torrent.LoadTorrentFile("./have/result.json")
	if err != nil {
		log.Fatal(err)
	}

	//tf, err := torrent.Open(inPath)
	//if err != nil {
	//	log.Fatal(err)
	//}

	// 判断文件夹是否存在
	if _, err := os.Stat(outPath); os.IsNotExist(err) {
		// 如果文件夹不存在，则创建文件夹
		err := os.MkdirAll(outPath, 0755)
		if err != nil {
			fmt.Println("Error creating folder:", err)
			return
		}
	}

	err = t.DownloadToFile(outPath + t.Name)
	if err != nil {
		log.Fatal(err)
	}

}
