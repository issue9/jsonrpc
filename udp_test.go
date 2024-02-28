// SPDX-FileCopyrightText: 2020-2024 caixw
//
// SPDX-License-Identifier: MIT

package jsonrpc

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/issue9/assert/v4"
	"github.com/issue9/unique/v2"
)

func TestUDP(t *testing.T) {
	const header = true
	a := assert.New(t, false)
	server := initServer(a)

	u := unique.NewString(10)
	go u.Serve(context.Background())

	srvExit := make(chan struct{}, 1)
	srvCtx, srvCancel := context.WithCancel(context.Background())
	srvT, err := NewUDPServerTransport(header, ":8089", time.Second)
	a.NotError(err).NotNil(srvT)
	srv := server.NewConn(srvT, nil)

	go func() {
		err := srv.Serve(srvCtx)
		a.True(errors.Is(err, context.Canceled))
		srvExit <- struct{}{}
	}()
	time.Sleep(500 * time.Millisecond) // 等待服务启动完成

	clientT, err := NewUDPClientTransport(header, ":8089", "", time.Second)
	a.NotError(err)
	client := NewServer(u.String).NewConn(clientT, nil)
	clientCtx, clientCancel := context.WithCancel(context.Background())
	clientExit := make(chan struct{}, 1)
	go func() {
		err := client.Serve(clientCtx)
		a.True(errors.Is(err, context.Canceled))
		clientExit <- struct{}{}
	}()
	time.Sleep(500 * time.Millisecond) // 等待服务启动完成

	f1Method := make(chan struct{}, 1)
	err = client.Send("f1", &inType{Age: 11}, func(result *outType) error {
		a.Equal(result.Age, 11)
		f1Method <- struct{}{}
		return nil
	})
	a.NotError(err)

	<-f1Method
	clientCancel()
	srvCancel()

	a.NotError(err)
	a.NotError(err)
	<-srvExit
	<-clientExit
}

func TestNewUDPClientTransport(t *testing.T) {
	a := assert.New(t, false)

	tp, err := NewUDPClientTransport(true, "8989", ":8989", time.Second)
	a.Error(err).Nil(tp)

	tp, err = NewUDPClientTransport(true, ":8989", "8989", time.Second)
	a.Error(err).Nil(tp)

	tp, err = NewUDPClientTransport(true, ":8989", "", time.Second)
	a.NotError(err).NotNil(tp)
}
