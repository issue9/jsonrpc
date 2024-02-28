// SPDX-FileCopyrightText: 2020-2024 caixw
//
// SPDX-License-Identifier: MIT

package jsonrpc

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
	"github.com/issue9/assert/v4"
)

var _ Transport = &websocketTransport{}

func TestNewWebsocketTransport(t *testing.T) {
	a := assert.New(t, false)

	rpcServer := initServer(a)

	ctx, cancel := context.WithCancel(context.Background())

	upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
	a.NotNil(upgrader)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		a.NotError(err).NotNil(conn)

		t := NewWebsocketTransport(conn)
		c := rpcServer.NewConn(t, nil)

		c.Serve(ctx)
	}))
	defer srv.Close()

	dialer := &websocket.Dialer{}
	conn, _, err := dialer.Dial(strings.Replace(srv.URL, "http", "ws", 1)+"/websocket", nil)
	a.NotError(err)
	client := rpcServer.NewConn(NewWebsocketTransport(conn), nil)

	err = client.Notify("f1", &inType{Age: 18})
	a.NotError(err)

	err = client.Send("f1", &inType{Age: 18}, func(out *outType) error {
		a.Equal(out.Age, 18)
		return nil
	})
	a.NotError(err)

	err = client.Send("f1", &inType{Age: 19, Last: "l"}, func(out *outType) error {
		a.Equal(out.Age, 19).Equal(out.Name, "l")
		return nil
	})
	a.NotError(err)

	cancel()
}
