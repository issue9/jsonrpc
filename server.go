// SPDX-License-Identifier: MIT

package jsonrpc

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sync"

	"github.com/issue9/autoinc"
)

// Server JSON RPC 服务实例
type Server struct {
	autoinc *autoinc.AutoInc
	servers sync.Map
	before  func(string) error
}

type handler struct {
	f       reflect.Value
	in, out reflect.Type
}

// NewServer 新的 Server 实例
func NewServer() *Server {
	return &Server{
		autoinc: autoinc.New(0, 1, 200),
	}
}

func (s *Server) id() *requestID {
	return &requestID{
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

var errType = reflect.TypeOf((*error)(nil)).Elem()

func newHandler(f interface{}) *handler {
	t := reflect.TypeOf(f)

	if t.Kind() != reflect.Func ||
		t.NumIn() != 3 ||
		t.In(0).Kind() != reflect.Bool ||
		t.In(1).Kind() != reflect.Ptr ||
		t.In(2).Kind() != reflect.Ptr ||
		!t.Out(0).Implements(errType) {
		panic(fmt.Sprintf("函数 %s 签名不正确", t.String()))
	}

	in := t.In(1).Elem()
	if in.Kind() == reflect.Func || in.Kind() == reflect.Ptr || in.Kind() == reflect.Invalid {
		panic(fmt.Sprintf("函数 %s 签名不正确", t.String()))
	}

	out := t.In(2).Elem()
	if out.Kind() == reflect.Func || out.Kind() == reflect.Ptr || out.Kind() == reflect.Invalid {
		panic(fmt.Sprintf("函数 %s 签名不正确", t.String()))
	}

	return &handler{
		f:   reflect.ValueOf(f),
		in:  in,
		out: out,
	}
}

// 作为服务端，根据参数查找和执行服务
func (s *Server) serve(t Transport) (func() error, error) {
	req := &request{}
	if err := t.Read(req); err != nil {
		return nil, s.writeError(t, CodeParseError, err, nil)
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
		return s.writeError(t, CodeMethodNotFound, errors.New("method not found"), nil)
	}

	h := f.(*handler)

	in := reflect.New(h.in)
	if err := json.Unmarshal(*req.Params, in.Interface()); err != nil {
		return s.writeError(t, CodeParseError, err, nil)
	}

	notify := req.ID == nil
	out := reflect.New(h.out)
	if errVal := h.f.Call([]reflect.Value{reflect.ValueOf(notify), in, out}); !errVal[0].IsNil() {
		return s.writeError(t, CodeInternalError, errVal[0].Interface().(error), nil)
	}

	if notify {
		return nil
	}

	data, err := json.Marshal(out.Interface())
	if err != nil {
		return err
	}

	resp := &response{
		Version: Version,
		Result:  (*json.RawMessage)(&data),
		ID:      req.ID,
	}
	return t.Write(resp)
}

func (s *Server) writeError(t Transport, code int, err error, data interface{}) error {
	resp := &response{
		Version: Version,
	}

	if err2, ok := err.(*Error); ok {
		resp.Error = err2
	} else {
		resp.Error = NewErrorWithData(code, err.Error(), data)
	}

	return t.Write(resp)
}
