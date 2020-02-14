// SPDX-License-Identifier: MIT

package jsonrpc

import (
	"context"
	"io/ioutil"
	"log"
	"net"
	"testing"
	"time"

	"github.com/issue9/assert"
)

var _ Transport = &streamTransport{}

func TestNewSocketTransport(t *testing.T) {
	a := assert.New(t)
	srv := initServer(a)

	exit := make(chan struct{}, 1)
	ctx, cancel := context.WithCancel(context.Background())

	srvConn, clientConn := net.Pipe()

	go func() {
		conn := srv.NewConn(NewSocketTransport(false, srvConn), log.New(ioutil.Discard, "", 0))
		conn.Serve(ctx)
		exit <- struct{}{}
	}()
	time.Sleep(500 * time.Millisecond) // 等待服务启动完成

	client := srv.NewConn(NewSocketTransport(false, clientConn), nil)

	err := client.Notify("f1", &inType{Age: 18})
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
	<-exit
}

func TestNewSocketTransport_withHeader(t *testing.T) {
	a := assert.New(t)
	srv := initServer(a)

	exit := make(chan struct{}, 1)
	ctx, cancel := context.WithCancel(context.Background())

	srvConn, clientConn := net.Pipe()

	go func() {
		conn := srv.NewConn(NewSocketTransport(true, srvConn), log.New(ioutil.Discard, "", 0))
		conn.Serve(ctx)
		exit <- struct{}{}
	}()
	time.Sleep(500 * time.Millisecond) // 等待服务启动完成

	client := srv.NewConn(NewSocketTransport(true, clientConn), nil)

	err := client.Notify("f1", &inType{Age: 18})
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
	<-exit
}
