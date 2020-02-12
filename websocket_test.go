// SPDX-License-Identifier: MIT

package jsonrpc

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
	"github.com/issue9/assert"
)

var _ Transport = &websocketTransport{}

func TestNewWebsocketTransport(t *testing.T) {
	a := assert.New(t)

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

	out := &outType{}
	err = client.Send("f1", &inType{Age: 18}, out)
	a.NotError(err).Equal(out.Age, 18)

	out = &outType{}
	err = client.Send("f1", &inType{Age: 19, Last: "l"}, out)
	a.NotError(err).Equal(out.Age, 19).Equal(out.Name, "l")

	// 检测抛出错误是否正确
	out = &outType{}
	err = client.Send("f2", &inType{Age: 19, Last: "l"}, out)
	err1, ok := err.(*Error)
	a.True(ok).Equal(err1.Code, CodeInvalidParams) // 由函数 f2 抛出的错误 *Error

	// 检测抛出错误是否正确
	out = &outType{}
	err = client.Send("f3", &inType{Age: 19, Last: "l"}, out)
	err1, ok = err.(*Error)
	a.True(ok).Equal(err1.Code, CodeInternalError) // 由函数 f3 抛出的普通错误

	cancel()
	// 触发 ctx 的退出事件
	err = client.Notify("f1", &inType{})
	a.NotError(err)
}
