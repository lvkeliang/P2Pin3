package network

import (
	"io"
	"net"
)

// Dial 函数，用于建立到指定地址的 TCP 连接，并返回连接对象和错误信息。
func Dial(address string) (net.Conn, error) {
	conn, err := net.Dial("tcp", address) // 建立到指定地址的 TCP 连接
	if err != nil {
		return nil, err // 返回空连接对象和错误信息
	}
	return conn, nil // 返回连接对象和空错误信息
}

// Listen 函数，用于在指定地址上监听 TCP 连接，并返回监听器对象和错误信息。
func Listen(address string) (net.Listener, error) {
	listener, err := net.Listen("tcp", address) // 在指定地址上监听 TCP 连接
	if err != nil {
		return nil, err // 返回空监听器对象和错误信息
	}
	return listener, nil // 返回监听器对象和空错误信息
}

// Accept 函数，用于从监听器对象中接受一个 TCP 连接，并返回连接对象和错误信息。
func Accept(listener net.Listener) (net.Conn, error) {
	conn, err := listener.Accept() // 从监听器对象中接受一个 TCP 连接
	if err != nil {
		return nil, err // 返回空连接对象和错误信息
	}
	return conn, nil // 返回连接对象和空错误信息
}

// Read 函数，用于从连接对象中读取数据，并返回读取到的字节数和错误信息。
func Read(conn net.Conn, buf []byte) (int, error) {
	n, err := conn.Read(buf) // 从连接对象中读取数据
	if err != nil {
		return 0, err // 返回读取到的字节数（0）和错误信息
	}
	return n, nil // 返回读取到的字节数和空错误信息
}

// Write 函数，用于向连接对象中写入数据，并返回写入的字节数和错误信息。
func Write(conn net.Conn, buf []byte) (int, error) {
	n, err := conn.Write(buf) // 向连接对象中写入数据
	if err != nil {
		return 0, err // 返回写入的字节数（0）和错误信息
	}
	return n, nil // 返回写入的字节数和空错误信息
}

// Close 函数，用于关闭连接或监听器对象，并返回错误信息。
func Close(closer io.Closer) error {
	err := closer.Close() // 关闭连接或监听器对象
	if err != nil {
		return err // 返回错误信息
	}
	return nil // 返回空错误信息
}
