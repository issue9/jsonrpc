// SPDX-License-Identifier: MIT

package jsonrpc

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"
)

// Server JSON RPC 服务实例
type Server struct {
	unique     func() string
	servers    sync.Map
	matchers   []matcher
	before     func(string) error
	errHandler func(*Error)
}

type matcher struct {
	matcher func(string) bool
	h       *handler
}

// NewServer 新的 Server 实例
//
// unique 用于生成唯一 ID 的方法。
func NewServer(unique func() string) *Server {
	return &Server{
		unique:   unique,
		matchers: []matcher{},
	}
}

func (s *Server) id() *ID { return &ID{alpha: s.unique()} }

// RegisterBefore 注册 Before 函数
//
// f 的原型如下：
//
//	func(method string)(err error)
//
// method RPC 服务名；
// 如果返回错误值，则会退出 RPC 调用，返回错误尽量采用 [Error] 类型；
//
// NOTE: 如果多次调用，仅最后次启作用。
func (s *Server) RegisterBefore(f func(method string) error) { s.before = f }

// Register 注册一个新的服务
//
// f 为处理服务的函数，其原型为以下方式：
//
//	func(notify bool, params, result pointer) error
//
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

// RegisterMatcher 注册服务名称通过函数判断的新服务
//
// m 为服务名称的匹配方法，其原型如下：
//
//	func(method string) bool
//
// 如果服务名称能正确匹配则返回 true。
//
// 通过 RegisterMatcher 注册的服务，其权重要低于 Register 注册的服务，
// 即一个服务名称只有在 Register 注册的列表中找不到，才会考虑通过在
// RegisterMatcher 注册的列表中查找。
func (s *Server) RegisterMatcher(m func(string) bool, f interface{}) {
	s.matchers = append(s.matchers, matcher{matcher: m, h: newHandler(f)})
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

// ErrHandler 指定请求数据的错误处理函数
//
// 仅针对请求数据，多次调用会相互覆盖。
func (s *Server) ErrHandler(h func(*Error)) { s.errHandler = h }

func (s *Server) read(t Transport) (*body, error) {
	req := &body{}
	if err := t.Read(req); err != nil {
		if errors.Is(err, os.ErrDeadlineExceeded) {
			return nil, nil
		}
		return nil, s.writeError(t, nil, CodeParseError, err, nil)
	}

	if req.isEmptyRequest() {
		return nil, s.writeError(t, nil, CodeInvalidRequest, errors.New("无效的请求内容"), nil)
	}

	return req, nil
}

func (s *Server) response(t Transport, req *body) error {
	if s.before != nil {
		if err := s.before(req.Method); err != nil {
			return s.writeError(t, req.ID, CodeMethodNotFound, err, nil)
		}
	}

	var h *handler
	if f, found := s.servers.Load(req.Method); found {
		h = f.(*handler)
	} else {
		for _, m := range s.matchers {
			if m.matcher(req.Method) {
				h = m.h
				break
			}
		}
		if h == nil {
			msg := fmt.Errorf("未找到对应的服务 %s", req.Method)
			return s.writeError(t, req.ID, CodeMethodNotFound, msg, nil)
		}
	}

	resp, err := h.call(req)
	if err != nil {
		return s.writeError(t, req.ID, CodeParseError, err, nil)
	}
	if resp == nil {
		return nil
	}
	return t.Write(resp)
}

func (s *Server) writeError(t Transport, id *ID, code int, err error, data interface{}) error {
	resp := &body{
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

// 作为客户端向服务端主动发送请求
func (s *Server) request(t Transport, notify bool, method string, in interface{}) (req *body, err error) {
	var params *json.RawMessage
	if in != nil {
		data, err := json.Marshal(in)
		if err != nil {
			return nil, err
		}
		params = (*json.RawMessage)(&data)
	}

	req = &body{
		Version: Version,
		Method:  method,
		Params:  params,
	}
	if !notify {
		req.ID = s.id()
	}

	if err = t.Write(req); err != nil {
		return nil, err
	}

	return req, nil
}
