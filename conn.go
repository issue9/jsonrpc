// SPDX-License-Identifier: MIT

package jsonrpc

import (
	"context"
	"encoding/json"
	"log"
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
	return conn.send(true, method, in, nil)
}

// Send 发送请求内容
//
// 发送数据 in 至服务，并获取返回的内容填充至 out。
func (conn *Conn) Send(method string, in, out interface{}) error {
	return conn.send(false, method, in, out)
}

func (conn *Conn) send(notify bool, method string, in, out interface{}) error {
	data, err := json.Marshal(in)
	if err != nil {
		return err
	}

	req := &request{
		Version: Version,
		Method:  method,
		Params:  (*json.RawMessage)(&data),
	}
	if !notify {
		req.ID = conn.server.id()
	}

	if err = conn.transport.Write(req); err != nil {
		return err
	}

	if notify {
		return nil
	}

	resp := &response{}
	if err = conn.transport.Read(resp); err != nil {
		return err
	}

	if resp.Error != nil {
		return resp.Error
	}

	if !req.ID.equal(resp.ID) {
		return NewError(CodeInvalidParams, "id not equal")
	}

	return json.Unmarshal(*resp.Result, out)
}

// Serve 作为服务端运行
//
// t 表示的是传输层的实例；
// ctx 可以用于中断当前的服务。但是需要注意，t 的 Read 和 Write
// 也有可能会阻塞整个服务，想要让 ctx 的取消启作用，还必须要有一定的机制从 Transport 中退出。
func (conn *Conn) Serve(ctx context.Context) error {
	defer func() {
		conn.transport.Close()
	}()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			f, err := conn.server.serve(conn.transport)
			if err != nil && conn.errlog != nil {
				conn.errlog.Println(err)
			}

			if err = f(); err != nil {
				conn.errlog.Println(err)
			}
		}
	}
}
