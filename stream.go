// SPDX-License-Identifier: MIT

package jsonrpc

import (
	"encoding/json"
	"io"
	"net"
	"sync"
)

// 定义基于流的传输层定义
type streamTransport struct {
	in *json.Decoder

	out    io.Writer
	outMux sync.Mutex
}

// NewSocketTransport 声明基于 socket 的 Transport 实例
//
// HTTP 和 websocket 有专门的实现方法
func NewSocketTransport(conn net.Conn) Transport {
	return NewStreamTransport(conn, conn)
}

// NewStreamTransport 返回基于流的 Transport 实例
func NewStreamTransport(in io.Reader, out io.Writer) Transport {
	return &streamTransport{
		in:  json.NewDecoder(in),
		out: out,
	}
}

func (s *streamTransport) Read(v interface{}) error {
	return s.in.Decode(v)
}

func (s *streamTransport) Write(v interface{}) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}

	s.outMux.Lock()
	defer s.outMux.Unlock()
	_, err = s.out.Write(data)
	return err
}
