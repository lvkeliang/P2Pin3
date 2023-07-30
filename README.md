# P2Pin2

## 如何使用

### 1.生成仿照torrent文件的json文件

参照以下示例代码以生成：

```go
        outPath := "./downloaded/"
	var hashmapPath = "./hashmap/hashmap.json"
	name := "[Sakurato] Kono Subarashii Sekai ni Bakuen wo! [12][AVC-8bit 1080p AAC][CHS].mp4"

	filePath := "./testdata/" + name
	newtorrent, err := torrent.NewTorrentFile(filePath, "http://localhost:8090/announce", 12*1024)
	if err != nil {
		log.Fatal(err)
	}

	err = newtorrent.SaveTorrentFile(filePath, "./have/"+name+".json", hashmapPath)
	if err != nil {
		log.Fatal(err)
	}
```

### 2.为server添加peer的地址并运行server

server用于为下载方返回拥有资源的peer的地址

使用以下代码运行server：

```sh
go run ./server/server.go
```

### 3.运行peer以监听并发送请求的资源：


修改peer.go的配置后运行以下代码以启动服务

```sh
go run ./peer/peer.go
```

可以更改peer.go中的port以启动多个服务

### 4.运行main以下载文件

修改main.go的配置以后运行以下代码以开始下载

```sh
go run ./cmd/main.go
```