// SPDX-FileCopyrightText: 2020-2024 caixw
//
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
//
// 如果需要使用 HTTP 的通讯模式，请使用 HTTPConn 对象。
type Conn struct {
	server    *Server
	errlog    *log.Logger
	transport Transport
	callbacks sync.Map
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
	_, err := conn.server.request(conn.transport, true, method, in)
	return err
}

// Send 发送请求内容
//
// 发送数据 in 至服务，在获得返回数据时，调用 callback 进行处理。
// callback 的原型如下：
//
//	func(result interface{}) error
//
// 参数 result 必须为一个指针，表示返回的数据对象；且函数返回一个 error。
func (conn *Conn) Send(method string, in, callback interface{}) error {
	req, err := conn.server.request(conn.transport, false, method, in)
	if err != nil {
		return err
	}

	cb := newCallback(callback)
	conn.callbacks.Store(req.ID.String(), cb)

	return nil
}

// Serve 运行服务
//
// 处理 Send 之后的数据或是作为服务端运行都需要调用此函数运行服务。
//
// ctx 可以用于中断当前的服务。但是需要注意，可能会被 Transport.Read 阻塞而无法退出，
// 所以在调用 cancel 之后，再下次对方有数据发送过来之后才能会退出。
// 作为客户端需要下一次的服务端数据下发才能退出，
// 而作为服务端需下一次的客户端请求才会真正退出。
// 用户可以自行实现在阻塞时返回 os.ErrDeadlineExceeded 解决此问题。
func (conn *Conn) Serve(ctx context.Context) (err error) {
	wg := &sync.WaitGroup{}
	defer wg.Wait()

	for {
		select {
		case <-ctx.Done():
			if err := conn.transport.Close(); err != nil {
				return err
			}
			return ctx.Err()
		default:
			body, err := conn.server.read(conn.transport)
			if err != nil {
				conn.printErr(err)
				continue
			}
			if body == nil {
				continue
			}

			wg.Add(1)
			go func() {
				defer wg.Done()
				conn.serve(body)
			}()
		}
	}
}

func (conn *Conn) serve(body *body) {
	if !body.isRequest() {
		if body.Error != nil {
			if conn.server.errHandler != nil {
				conn.server.errHandler(body.Error)
			}
		} else if f, found := conn.callbacks.Load(body.ID.String()); found {
			if err := f.(*callback).call(body); err != nil {
				conn.printErr(err)
			}
			conn.callbacks.Delete(body.ID.String())
		} else {
			conn.printErr(fmt.Sprintf("未找到 %s 的回调函数,%+v\n", body.ID, body))
		}
	} else {
		if err := conn.server.response(conn.transport, body); err != nil {
			conn.printErr(err)
		}
	}
}

func (conn *Conn) printErr(v interface{}) {
	if conn.errlog != nil {
		conn.errlog.Println(v)
	}
}
