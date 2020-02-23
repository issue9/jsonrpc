// SPDX-License-Identifier: MIT

package jsonrpc

import (
	"context"
	"fmt"
	"log"
	"sync"
)

// Conn JSON RPC 连接对象
//
// json-rpc 客户端和服务端是对等的，两者都使用 conn 初始化。
type Conn struct {
	server    *Server
	errlog    *log.Logger
	transport Transport
}

// NewConn 创建长链接的 JSON RPC 实例
//
// t 表示传输层的操作实例；
// errlog 表示在 serveHTTP 和 Serve 中部分不会中断执行的错误输出。
// 如果为空，则不会输出这些错误。
func (s *Server) NewConn(t Transport, errlog *log.Logger) *Conn {
	return &Conn{
		server:    s,
		transport: t,
		errlog:    errlog,
	}
}

// Notify 发送通知信息
//
// 仅发送 in 至服务端，会忽略服务端返回的信息。
func (conn *Conn) Notify(method string, in interface{}) error {
	return conn.request(true, method, in, nil)
}

// Send 发送请求内容
//
// 发送数据 in 至服务，并获取返回的内容填充至 out。
func (conn *Conn) Send(method string, in, out interface{}) error {
	return conn.request(false, method, in, out)
}

func (conn *Conn) request(notify bool, method string, in, out interface{}) error {
	return conn.server.request(conn.transport, notify, method, in, out)
}

// Serve 作为服务端运行
//
// t 表示的是传输层的实例；
// ctx 可以用于中断当前的服务。但是需要注意，可能会被 Transport.Read 阻塞而无法退出，
// 所以在调用 cancel 之后，可能还需要向 Conn 发送一条任意指令才行。
func (conn *Conn) Serve(ctx context.Context) (err error) {
	wg := &sync.WaitGroup{}

	defer func() {
		wg.Wait()
		if err2 := conn.transport.Close(); err2 != nil {
			if err != nil {
				err = fmt.Errorf("在抛出错误 %w 时，再次发生了错误 %v", err, err2)
			} else {
				err = err2
			}
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			f, err := conn.server.read(conn.transport)
			if err != nil && conn.errlog != nil {
				conn.errlog.Println(err)
				continue
			}
			if f == nil {
				continue
			}

			wg.Add(1)
			go func() {
				defer wg.Done()
				if err = f(); err != nil {
					conn.errlog.Println(err)
				}
			}()
		}
	}
}
