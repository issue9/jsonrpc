// SPDX-License-Identifier: MIT

package jsonrpc

import (
	"errors"
	"sync"

	"github.com/issue9/autoinc"
)

// Server JSON RPC 服务实例
type Server struct {
	autoinc *autoinc.AutoInc
	servers sync.Map
	before  func(string) error
}

// NewServer 新的 Server 实例
func NewServer() *Server {
	return &Server{
		autoinc: autoinc.New(0, 1, 200),
	}
}

func (s *Server) id() *ID {
	return &ID{
		isNumber: true,
		number:   s.autoinc.MustID(),
	}
}

// RegisterBefore 注册 Before 函数
//
// f 的原型如下：
//  func(method string)(err error)
// method RPC 服务名；
// 如果返回错误值，则会退出 RPC 调用，返回错误尽量采用 *Error 类型；
//
// NOTE: 如果多次调用，仅最后次启作用。
func (s *Server) RegisterBefore(f func(method string) error) {
	s.before = f
}

// Register 注册一个新的服务
//
// f 为处理服务的函数，其原型为以下方式：
//  func(notify bool, params, result pointer) error
// 其中 notify 表示是否为通知类型的请求；params 为用户请求的对象；
// result 为返回给用户的数据对象；error 则为处理出错是的返回值。
// params 和 result 必须为指针类型。
//
// 返回值表示是否添加成功，在已经存在相同值时，会添加失败。
//
// NOTE: 如果 f 的签名不正确，则会直接 panic
func (s *Server) Register(method string, f interface{}) bool {
	if s.Exists(method) {
		return false
	}

	s.servers.Store(method, newHandler(f))
	return true
}

// Exists 是否已经存在相同的方法名
func (s *Server) Exists(method string) bool {
	_, found := s.servers.Load(method)
	return found
}

// Registers 注册多个服务方法
//
// 如果已经存在相同的方法名，则会直接 panic
func (s *Server) Registers(methods map[string]interface{}) {
	for method, f := range methods {
		if !s.Register(method, f) {
			panic("已经存在相同的方法：" + method)
		}
	}
}

// 可能返回 nil,nil 的情况
//
// 如果返回的函数为 nil，表示不需要调用函数，即已经输出了错误信息。
func (s *Server) read(t Transport) (func() error, error) {
	req := &request{}
	if err := t.Read(req); err != nil {
		// 解析 request 出错，没有 ID 返回。
		return nil, s.writeError(t, nil, CodeParseError, err, nil)
	}

	if req.isEmpty() {
		return nil, s.writeError(t, nil, CodeInvalidRequest, errors.New("invalid request"), nil)
	}

	return func() error {
		return s.response(t, req)
	}, nil
}

func (s *Server) response(t Transport, req *request) error {
	if s.before != nil {
		if err := s.before(req.Method); err != nil {
			return err
		}
	}

	f, found := s.servers.Load(req.Method)
	if !found {
		return s.writeError(t, req.ID, CodeMethodNotFound, errors.New("method not found"), nil)
	}

	resp, err := f.(*handler).call(req)
	if err != nil {
		return s.writeError(t, req.ID, CodeParseError, err, nil)
	}
	if resp == nil {
		return nil
	}
	return t.Write(resp)
}

func (s *Server) writeError(t Transport, id *ID, code int, err error, data interface{}) error {
	resp := &response{
		Version: Version,
		ID:      id,
	}

	if err2, ok := err.(*Error); ok {
		resp.Error = err2
	} else {
		resp.Error = NewErrorWithData(code, err.Error(), data)
	}

	return t.Write(resp)
}
