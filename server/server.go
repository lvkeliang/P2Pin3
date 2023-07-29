package main

import (
	"encoding/json"
	"net/http"
)

type Peer struct {
	ID   string `json:"id"`
	IP   string `json:"ip"`
	Port int    `json:"port"`
}

type TrackerResponse struct {
	Peers []Peer `json:"peers"`
}

func handleRequest(w http.ResponseWriter, r *http.Request) {
	// 构造响应数据
	response := TrackerResponse{
		Peers: []Peer{
			{ID: "peer1", IP: "127.0.0.1", Port: 8096},
			{ID: "peer2", IP: "127.0.0.1", Port: 8097},
		},
	}

	// 将响应数据编码为 JSON 字符串
	data, err := json.Marshal(response)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 设置响应头
	w.Header().Set("Content-Type", "application/json")

	// 发送响应数据
	w.Write(data)
}

func main() {
	http.HandleFunc("/announce", handleRequest)
	err := http.ListenAndServe(":8090", nil)
	if err != nil {
		panic(err)
	}
}
