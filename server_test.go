// SPDX-FileCopyrightText: 2020-2024 caixw
//
// SPDX-License-Identifier: MIT

package jsonrpc

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"testing"

	"github.com/issue9/assert/v4"
	"github.com/issue9/unique/v2"
)

var (
	_ Transport = &httpClientTransport{}
	_ Transport = &httpTransport{}
)

var uniqueID = make(chan string, 100)

func init() {
	go func() {
		for i := 0; i < 1000000; i++ {
			uniqueID <- strconv.Itoa(i)
		}
	}()
}

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
	u := unique.NewString(10)
	go u.Serve(context.Background())

	srv := NewServer(u.String)
	a.NotNil(srv)

	a.True(srv.Register("f1", f1))
	a.True(srv.Register("f2", f2))
	a.True(srv.Register("f3", f3))

	a.False(srv.Register("f3", f3))

	srv.RegisterMatcher(func(m string) bool {
		return strings.HasPrefix(m, "ok/")
	}, f1)

	return srv
}

func TestServer_read(t *testing.T) {
	a := assert.New(t, false)
	srv := initServer(a)

	data := []*struct {
		req string
		err int // 0 表示无错误，其它表示对应的 Error.Code
	}{
		{
			req: ``,
			err: CodeParseError,
		},

		{
			req: `{"jsonrpc"`,
			err: CodeParseError,
		},

		{
			req: `{}`,
			err: CodeInvalidRequest,
		},

		{
			req: `{"jsonrpc":"2.0"}`,
		},
	}

	in := new(bytes.Buffer)
	out := new(bytes.Buffer)
	for i, item := range data {
		in.Reset()
		out.Reset()
		in.WriteString(item.req)
		f, err := srv.read(NewStreamTransport(false, in, out, nil))
		a.NotError(err)

		if item.err == 0 {
			a.NotNil(f, "nil @ %d", i)
		} else {
			a.Nil(f)

			resp := &body{}
			a.NotError(json.Unmarshal(out.Bytes(), resp))
			a.NotNil(resp.Error).
				Equal(resp.Error.Code, item.err, "not equal v1=%v,v2=%v @ %d", resp.Error.Code, item.err, i)
		}
	}
}

func TestServer_response(t *testing.T) {
	a := assert.New(t, false)
	srv := initServer(a)
	srv.RegisterBefore(func(method string) error {
		if method == "b2" {
			return NewError(-32111, "not found")
		}
		if method == "b5" {
			return errors.New("f5")
		}
		return nil
	})

	data := []*struct {
		err    int // 0 表示无错误，其它表示对应的 Error.Code
		in     *inType
		out    *outType
		method string
	}{
		{ // in==nil
			out:    &outType{Age: 0},
			method: "f1",
		},
		{ // 正常
			in:     &inType{Age: 18},
			out:    &outType{Age: 18},
			method: "f1",
		},
		{ // f2 抛出错误
			in:     &inType{Age: 18},
			method: "f2",
			err:    CodeInvalidParams,
		},
		{ // 触发 before
			in:     &inType{Age: 18},
			method: "b2",
			err:    -32111,
		},
		{ // 触发 before，普通错误
			in:     &inType{Age: 18},
			method: "b5",
			err:    CodeMethodNotFound,
		},
		{ // 不存在
			in:     &inType{Age: 18},
			method: "not-exists",
			err:    CodeMethodNotFound,
		},

		{ // registerMatcher，对应 f1
			in:     &inType{Age: 18},
			out:    &outType{Age: 18},
			method: "ok/111",
		},
		{ // 不存在
			in:     &inType{Age: 18},
			method: "ok111",
			err:    CodeMethodNotFound,
		},
	}

	in := new(bytes.Buffer)
	out := new(bytes.Buffer)
	for i, item := range data {
		in.Reset()
		out.Reset()

		data, err := json.Marshal(item.in)
		a.NotError(err)
		req := &body{
			Version: Version,
			ID:      srv.id(),
			Method:  item.method,
			Params:  (*json.RawMessage)(&data),
		}
		data, err = json.Marshal(req)
		a.NotError(err)
		_, err = in.Write(data)
		a.NotError(err)

		transport := NewStreamTransport(false, in, out, nil)
		ret, err := srv.read(transport)
		a.NotError(err).NotNil(ret)
		a.NotError(srv.response(transport, ret))

		resp := &body{}
		a.NotError(json.Unmarshal(out.Bytes(), resp))

		if item.err == 0 {
			a.Nil(resp.Error, "not nil %s @ %d", resp.Error, i)

			out := &outType{}
			a.NotError(json.Unmarshal(*resp.Result, out))
			a.Equal(item.out, out)
		} else {
			a.NotNil(resp.Error).
				Equal(resp.Error.Code, item.err, "not equal\nv1=%v\nv2=%v\n@ %d", resp.Error.Code, item.err, i)
		}
	}
}

func TestServer_Registers(t *testing.T) {
	u := unique.NewString(10)
	go u.Serve(context.Background())

	a := assert.New(t, false)
	srv := NewServer(u.String)
	a.NotNil(srv)

	a.NotPanic(func() {
		srv.Registers(map[string]interface{}{
			"f1": f1,
			"f2": f2,
		})
	})

	srv = NewServer(u.String)
	a.NotNil(srv)
	a.Panic(func() {
		srv.Registers(map[string]interface{}{
			"f1": f1,
			"f2": initServer, // 签名不正确
		})
	})

	srv = NewServer(u.String)
	a.NotNil(srv)
	a.Panic(func() {
		a.True(srv.Register("f1", f1))
		srv.Registers(map[string]interface{}{
			"f1": f1, // 重名
		})
	})
}
