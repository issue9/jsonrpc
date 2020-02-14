// SPDX-License-Identifier: MIT

package jsonrpc

import (
	"sync"

	"github.com/gorilla/websocket"
)

type websocketTransport struct {
	conn *websocket.Conn

	writeMux sync.Mutex
}

// NewWebsocketTransport 声明基于 websocket 的 Transport 实例
func NewWebsocketTransport(conn *websocket.Conn) Transport {
	return &websocketTransport{conn: conn}
}

func (s *websocketTransport) Read(v interface{}) error {
	return s.conn.ReadJSON(v)
}

func (s *websocketTransport) Write(v interface{}) error {
	s.writeMux.Lock()
	defer s.writeMux.Unlock()

	return s.conn.WriteJSON(v)
}

func (s *websocketTransport) Close() error {
	return s.conn.Close()
}
