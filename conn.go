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
// 发送数据 in 至服务，在返回数据时，调用 callback 进行处理。
// callback 的原型如下：
//  func(result interface{}) error
// 参数 result 必须为一个指针，且函数返回一个 error。
func (conn *Conn) Send(method string, in, callback interface{}) error {
	req, err := conn.server.request(conn.transport, false, method, in)
	if err != nil {
		return err
	}

	cb := newCallback(callback)
	conn.callbacks.Store(req.ID, cb)

	return nil
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
			body, err := conn.server.read(conn.transport)
			if err != nil {
				conn.printErr(err)
				continue
			}

			conn.serve(body, wg)
		}
	}
}

func (conn *Conn) serve(body *body, wg *sync.WaitGroup) {
	wg.Add(1)

	if !body.isRequest() {
		go func() {
			defer wg.Done()

			if f, found := conn.callbacks.Load(body.ID); found {
				if err := f.(*callback).call(body); err != nil {
					conn.printErr(err)
				}
			} else {
				conn.printErr(fmt.Sprintf("未找到 %s 的处理函数\n", body.ID))
			}
		}()
	} else {
		go func() {
			defer wg.Done()

			if err := conn.server.response(conn.transport, body); err != nil {
				conn.printErr(err)
			}
		}()
	}
}

func (conn *Conn) printErr(v interface{}) {
	if conn.errlog != nil {
		conn.errlog.Println(v)
	}
}
