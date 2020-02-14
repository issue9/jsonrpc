// SPDX-License-Identifier: MIT

package jsonrpc

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"

	"github.com/issue9/assert"
)

// 用于测试的数据类型
type (
	inType struct {
		Last  string `json:"last"`
		First string `json:"first"`
		Age   int
	}

	outType struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}
)

var (
	f1 = func(notify bool, params *inType, result *outType) error {
		if notify {
			return nil
		}

		result.Name = params.First + params.Last
		result.Age = params.Age
		return nil
	}

	// 抛出 Error
	f2 = func(notify bool, params *inType, result *outType) error {
		return NewError(CodeInvalidParams, "invalid params")
	}

	// 抛出普通错误
	f3 = func(notify bool, params *inType, result *outType) error {
		return errors.New("error")
	}
)

func initServer(a *assert.Assertion) *Server {
	srv := NewServer()
	a.NotNil(srv)

	a.True(srv.Register("f1", f1))
	a.True(srv.Register("f2", f2))
	a.True(srv.Register("f3", f3))

	a.False(srv.Register("f3", f3))

	return srv
}

func TestServer_read(t *testing.T) {
	a := assert.New(t)
	srv := initServer(a)

	write := func(w *bytes.Buffer, method string, obj *inType) {
		data, err := json.Marshal(obj)
		a.NotError(err)
		req := &request{
			Version: Version,
			ID:      srv.id(),
			Method:  method,
			Params:  (*json.RawMessage)(&data),
		}
		data, err = json.Marshal(req)
		a.NotError(err)
		a.NotError(w.Write(data))
	}

	read := func(r *bytes.Buffer, obj *outType) {
		resp := &response{}
		a.NotError(json.Unmarshal(r.Bytes(), resp))
		a.NotError(resp.Error)
		a.NotError(json.Unmarshal(*resp.Result, obj))
	}

	srv.RegisterBefore(func(method string) error {
		if method == "f2" {
			return NewError(CodeMethodNotFound, "not found")
		}
		return nil
	})

	in := new(bytes.Buffer)
	out := new(bytes.Buffer)
	write(in, "f1", &inType{Last: "l", First: "F"})
	f, err := srv.read(NewStreamTransport(in, out, nil))
	a.NotError(err).NotNil(f)
	a.NotError(f())
	o := &outType{}
	read(out, o)
	a.Equal(o.Name, "Fl").Empty(o.Age)

	// 触发 before
	in.Reset()
	out.Reset()
	write(in, "f2", &inType{Last: "l", First: "F"})
	f, err = srv.read(NewStreamTransport(in, out, nil))
	a.NotError(err).NotNil(f)
	err = f() // before 此处调用
	err2, ok := err.(*Error)
	a.True(ok).Equal(err2.Code, CodeMethodNotFound)

	// 写入任意数据，read 返回两个值都为 nil
	in.Reset()
	out.Reset()
	in.WriteString("xxx:wes-->")
	f, err = srv.read(NewStreamTransport(in, out, nil))
	a.NotError(err).Nil(f)
}

func TestServer_Registers(t *testing.T) {
	a := assert.New(t)
	srv := NewServer()
	a.NotError(srv)

	a.NotPanic(func() {
		srv.Registers(map[string]interface{}{
			"f1": f1,
			"f2": f2,
		})
	})

	srv = NewServer()
	a.NotError(srv)
	a.Panic(func() {
		srv.Registers(map[string]interface{}{
			"f1": f1,
			"f2": initServer, // 签名不正确
		})
	})

	srv = NewServer()
	a.NotError(srv)
	a.Panic(func() {
		a.True(srv.Register("f1", f1))
		srv.Registers(map[string]interface{}{
			"f1": f1, // 重名
		})
	})
}

func TestNewHandler(t *testing.T) {
	a := assert.New(t)

	type f func()

	a.Panic(func() {
		newHandler(5)
	})

	// 参数数量不正确
	a.Panic(func() {
		newHandler(func(*int) error { return nil })
	})

	// 参数类型不正确
	a.Panic(func() {
		newHandler(func(bool, *f, *int) error { return nil })
	})

	// 参数类型不正确
	a.Panic(func() {
		newHandler(func(bool, *int, *f) error { return nil })
	})

	// 参数类型不正确
	a.Panic(func() {
		newHandler(func(*bool, *int, *int) error { return nil })
	})

	// 返回值不正确
	a.Panic(func() {
		newHandler(func(bool, *int, *int) int { return 0 })
	})

	// 返回值不正确
	a.NotPanic(func() {
		newHandler(func(bool, *int, *int) *Error { return nil })
	})

	// 正常签名
	a.NotPanic(func() {
		newHandler(func(bool, *int, *int) error { return nil })
	})
}
