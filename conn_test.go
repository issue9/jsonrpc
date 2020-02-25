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

func TestConn_Serve(t *testing.T) {
	a := assert.New(t)
	srv := initServer(a)
	srvConn, clientConn := net.Pipe()

	srvExit := make(chan struct{}, 1)
	srvCtx, srvCancel := context.WithCancel(context.Background())
	go func() {
		conn := srv.NewConn(NewSocketTransport(false, srvConn), log.New(ioutil.Discard, "", 0))
		err := conn.Serve(srvCtx)
		a.Equal(err, context.Canceled)
		srvExit <- struct{}{}
	}()
	time.Sleep(500 * time.Millisecond) // 等待服务启动完成

	clientExit := make(chan struct{}, 1)
	clientCtx, clientCancel := context.WithCancel(context.Background())
	client := srv.NewConn(NewSocketTransport(false, clientConn), nil)
	go func() {
		err := client.Serve(clientCtx)
		a.Equal(err, context.Canceled)
		clientExit <- struct{}{}
	}()
	time.Sleep(500 * time.Millisecond) // 等待服务启动完成

	err := client.Notify("f1", &inType{Age: 18})
	a.NotError(err)

	err = client.Send("f1", &inType{Age: 18}, func(out *outType) error {
		a.Equal(out.Age, 18)
		return nil
	})
	a.NotError(err)

	err = client.Send("f1", &inType{Age: 19, Last: "l"}, func(out *outType) error {
		a.NotError(err).Equal(out.Age, 19).Equal(out.Name, "l")
		return nil
	})
	a.NotError(err)

	srvCancel()
	clientCancel()
	// 触发 srvCtx 的退出事件
	//err = client.Notify("f1", &inType{})
	//a.NotError(err)
	<-srvExit
	<-clientExit
}
