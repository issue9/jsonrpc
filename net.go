// SPDX-License-Identifier: MIT

package jsonrpc

import (
    "encoding/json"
	"io"
	"net"
	"sync"
)

type netTransport struct {
	in *json.Decoder

	out    io.Writer
	outMux sync.Mutex
}

// NewNetTransport 声明基于网络通讯的 Transport 实例
//
// HTTP 和 websocket 有专门的实现方法
func NewNetTransport(conn net.Conn) Transport {
	return &netTransport{
		in:  json.NewDecoder(conn),
		out: conn,
	}
}

func (s *netTransport) Read(v interface{}) error {
	return s.in.Decode(v)
}

func (s *netTransport) Write(v interface{}) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}

	s.outMux.Lock()
	defer s.outMux.Unlock()
	_, err = s.out.Write(data)
	return err
}
