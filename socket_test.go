// SPDX-License-Identifier: MIT

package jsonrpc

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/issue9/assert"
)

var _ Transport = &socketTransport{}

func TestConn_Serve(t *testing.T) {
	a := assert.New(t)
	conn := initConn(a, nil)
	exit := make(chan struct{}, 1)
	ctx, cancel := context.WithCancel(context.Background())

	l, err := net.Listen("tcp", ":8080")
	a.NotError(err)

	go func() {
		c, err := l.Accept()
		a.NotError(err)
		conn.Serve(ctx, NewSocketTransport(c))
		exit <- struct{}{}
	}()
	time.Sleep(500 * time.Millisecond) // 等待服务启动完成

	dialConn, err := net.Dial("tcp", ":8080")
	a.NotError(err).NotNil(dialConn)
	client := NewConn(nil)

	err = client.Notify(NewSocketTransport(dialConn), "f1", &inType{Age: 18})
	a.NotError(err)

	out := &outType{}
	err = client.Send(NewSocketTransport(dialConn), "f1", &inType{Age: 18}, out)
	a.NotError(err).Equal(out.Age, 18)

	out = &outType{}
	err = client.Send(NewSocketTransport(dialConn), "f1", &inType{Age: 19, Last: "l"}, out)
	a.NotError(err).Equal(out.Age, 19).Equal(out.Name, "l")

	cancel()
	// 触发 ctx 的退出事件
	err = client.Notify(NewSocketTransport(dialConn), "f1", &inType{})
	a.NotError(err)
	<-exit
}
