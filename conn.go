// SPDX-License-Identifier: MIT

package jsonrpc

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"reflect"
	"sync"

	"github.com/issue9/autoinc"
)

// Conn JSON RPC 连接对象
//
// json-rpc 客户端和服务端是对等的，两者都使用 conn 初始化。
type Conn struct {
	errlog  *log.Logger
	servers sync.Map
	autoinc *autoinc.AutoInc
}

type handler struct {
	f       reflect.Value
	in, out reflect.Type
}

// NewConn 声明新的 Conn 实例
//
// errlog 表示在 serveHTTP 和 Serve 中部分不会中断执行的错误输出。
// 如果为空，则不会输出这些错误。
func NewConn(errlog *log.Logger) *Conn {
	return &Conn{
		errlog:  errlog,
		autoinc: autoinc.New(1, 1, 1000),
	}
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
func (conn *Conn) Register(method string, f interface{}) bool {
	if conn.Exists(method) {
		return false
	}

	conn.servers.Store(method, newHandler(f))
	return true
}

// Exists 是否已经存在相同的方法名
func (conn *Conn) Exists(method string) bool {
	_, found := conn.servers.Load(method)
	return found
}

// Registers 注册多个服务方法
//
// 如果已经存在相同的方法名，则会直接 panic
func (conn *Conn) Registers(methods map[string]interface{}) {
	for method, f := range methods {
		if !conn.Register(method, f) {
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
		panic("函数签名不正确")
	}

	in := t.In(1).Elem()
	if in.Kind() == reflect.Func || in.Kind() == reflect.Ptr || in.Kind() == reflect.Invalid {
		panic("函数签名不正确")
	}

	out := t.In(2).Elem()
	if out.Kind() == reflect.Func || out.Kind() == reflect.Ptr || out.Kind() == reflect.Invalid {
		panic("函数签名不正确")
	}

	return &handler{
		f:   reflect.ValueOf(f),
		in:  in,
		out: out,
	}
}

// Notify 发送通知信息
//
// 仅发送 in 至服务端，会忽略服务端返回的信息。
func (conn *Conn) Notify(t Transport, method string, in interface{}) error {
	return conn.send(t, true, method, in, nil)
}

// Send 发送请求内容
//
// 发送数据 in 至服务，并获取返回的内容填充至 out。
func (conn *Conn) Send(t Transport, method string, in, out interface{}) error {
	return conn.send(t, false, method, in, out)
}

func (conn *Conn) send(t Transport, notify bool, method string, in, out interface{}) error {
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
		req.ID = &requestID{isNumber: true, number: conn.autoinc.MustID()}
	}

	if err = t.Write(req); err != nil {
		return err
	}

	if notify {
		return nil
	}

	resp := &response{}
	if err = t.Read(resp); err != nil {
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
func (conn *Conn) Serve(ctx context.Context, t Transport) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			if err := conn.serve(t); err != nil && conn.errlog != nil {
				conn.errlog.Println(err)
			}
		}
	}
}

func (conn *Conn) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	t := NewHTTPTransport(w, r)
	if err := conn.serve(t); err != nil && conn.errlog != nil {
		conn.errlog.Println(err)
	}
}

// 作为服务端，根据参数查找和执行服务
func (conn *Conn) serve(t Transport) error {
	req := &request{}
	if err := t.Read(req); err != nil {
		return conn.writeError(t, CodeParseError, err, nil)
	}

	f, found := conn.servers.Load(req.Method)
	if !found {
		return conn.writeError(t, CodeMethodNotFound, errors.New("method not found"), nil)
	}
	h := f.(*handler)

	notify := req.ID == nil
	in := reflect.New(h.in)
	if err := json.Unmarshal(*req.Params, in.Interface()); err != nil {
		return conn.writeError(t, CodeParseError, err, nil)
	}

	out := reflect.New(h.out)
	if errVal := h.f.Call([]reflect.Value{reflect.ValueOf(notify), in, out}); !errVal[0].IsNil() {
		return conn.writeError(t, CodeInternalError, errVal[0].Interface().(error), nil)
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

func (conn *Conn) writeError(t Transport, code int, err error, data interface{}) error {
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
