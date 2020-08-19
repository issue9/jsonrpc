// SPDX-License-Identifier: MIT

package jsonrpc

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/issue9/assert"
)

func TestUDP(t *testing.T) {
	a := assert.New(t)
	header := true
	server := initServer(a)

	srvExit := make(chan struct{}, 1)
	srvCtx, srvCancel := context.WithCancel(context.Background())
	srvT, err := NewUDPServerTransport(header, ":8089")
	a.NotError(err).NotNil(srvT)
	srv := server.NewConn(srvT, nil)

	go func() {
		err := srv.Serve(srvCtx)
		a.True(errors.Is(err, context.Canceled))
		srvExit <- struct{}{}
	}()
	time.Sleep(500 * time.Millisecond) // 等待服务启动完成

	clientT, err := NewUDPClientTransport(header, ":8089", "")
	a.NotError(err)
	client := NewServer().NewConn(clientT, nil)
	clientCtx, clientCancel := context.WithCancel(context.Background())
	clientExit := make(chan struct{}, 1)
	go func() {
		err := client.Serve(clientCtx)
		a.True(errors.Is(err, context.Canceled))
		clientExit <- struct{}{}
	}()
	time.Sleep(500 * time.Millisecond) // 等待服务启动完成

	f1Method := make(chan struct{}, 1)
	client.Send("f1", &inType{Age: 11}, func(result *outType) error {
		a.Equal(result.Age, 11)
		f1Method <- struct{}{}
		return nil
	})

	<-f1Method
	clientCancel()
	srvCancel()

	err = client.Notify("f1", &inType{}) // 触发 srvCtx 的退出事件
	a.NotError(err)
	err = srv.Notify("f1", nil) // 触发 srvCtx 的退出事件
	a.NotError(err)
	<-srvExit
	<-clientExit
}

func TestNewUDPClientTransport(t *testing.T) {
	a := assert.New(t)

	tp, err := NewUDPClientTransport(true, "8989", ":8989")
	a.Error(err).Nil(tp)

	tp, err = NewUDPClientTransport(true, ":8989", "8989")
	a.Error(err).Nil(tp)

	tp, err = NewUDPClientTransport(true, ":8989", "")
	a.NotError(err).NotNil(tp)
}
