// SPDX-License-Identifier: MIT

package jsonrpc

import (
	"io"
	"net"
	"sync"
)

type udpReadWriter struct {
	conn *net.UDPConn

	addr    *net.UDPAddr
	addrMux sync.RWMutex
}

func (conn *udpReadWriter) Read(p []byte) (n int, err error) {
	var addr *net.UDPAddr
	n, addr, err = conn.conn.ReadFromUDP(p)

	conn.addrMux.Lock()
	conn.addr = addr
	conn.addrMux.Unlock()

	return n, err
}

func (conn *udpReadWriter) Write(b []byte) (int, error) {
	conn.addrMux.RLock()
	defer conn.addrMux.RUnlock()
	return conn.conn.WriteToUDP(b, conn.addr)
}

func (conn *udpReadWriter) Close() error {
	return conn.conn.Close()
}

// NewUDPTransport 创建 UDP 传输层
//
// UDP 作为服务端是无状态的，在客户端发送一次请求之后，才能发送信息给客户端，
// 且之后如果有新的客户端请求过来，则发送的目标不地址也会变化。在多客户端环境中，
// 服务端如果有下发数据的行为，接收方是无法保证的。
//
// header 表示是否需要输出报头内容目前报头包含了长度和编码两个字段，
// 如果不包含报头，则是一段合法的 JSON 内容。
// connected 表示 conn 是否是有状态的，如果是调用 net.ListenUDP 生成的实例，是无状态的；
// net.DialUDP 返回的则是有状态的连接。
func NewUDPTransport(header bool, conn *net.UDPConn, connected bool) Transport {
	var rw io.ReadWriteCloser = conn
	if !connected {
		rw = &udpReadWriter{conn: conn}
	}
	return NewStreamTransport(header, rw, rw, func() error { return rw.Close() })
}

// NewUDPServerTransport 声明用于服务的 UDP Transport 接口
//
// 这是对 NewUDPTransport 的二次封装，返回适用于服务端的接口实例，
// 其中的 conn 参数由 net.ListenUDP 创建，而 connected 统一为 false。
func NewUDPServerTransport(header bool, addr string) (Transport, error) {
	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return nil, err
	}

	c, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		return nil, err
	}

	return NewUDPTransport(header, c, false), nil
}

// NewUDPClientTransport 声明用于客户的 UDP Transport 接口
//
// 这是对 NewUDPTransport 的二次封装，返回适用于客户端的接口实例，
// 其中的 conn 参数由 net.DialUDP 创建，而 connected 统一为 true。
//
// raddr 用于指定服务端地址；laddr 用于指定本地地址，可以为空值。
func NewUDPClientTransport(header bool, raddr, laddr string) (Transport, error) {
	remote, err := net.ResolveUDPAddr("udp", raddr)
	if err != nil {
		return nil, err
	}

	var local *net.UDPAddr
	if laddr != "" {
		local, err = net.ResolveUDPAddr("udp", laddr)
		if err != nil {
			return nil, err
		}
	}

	conn, err := net.DialUDP("udp", local, remote)
	if err != nil {
		return nil, err
	}

	return NewUDPTransport(header, conn, true), nil
}
