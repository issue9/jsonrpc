// SPDX-License-Identifier: MIT

package jsonrpc

import (
	"encoding/json"
	"errors"
	"math"
	"testing"

	"github.com/issue9/assert"
)

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

func TestHandler_call(t *testing.T) {
	a := assert.New(t)

	data := []*struct {
		h      *handler
		in     string // 输入的 request.Params 数据，为 JSON 格式数据
		out    string // call 返回的 response 实例内容，为 JSON 格式数据
		err    int    // 错误代码，对应 Error.Code 字段，如果为 0 表示没有错误，-1 表示普通错误
		notify bool   // 是否请求为通知类型
	}{
		{
			h:   newHandler(func(bool, *int, *int) error { return nil }),
			in:  "5",
			out: "0",
		},

		{
			h: newHandler(func(notify bool, in *int, out *int) error {
				*out = (*in) + 1
				return nil
			}),
			in:  "5",
			out: "6",
		},

		{ // 处理函数返回的普通错误
			h:   newHandler(func(bool, *int, *int) error { return errors.New("error") }),
			in:  "5",
			out: "0",
			err: CodeInternalError,
		},

		{ // 处理函数返回的 *Error
			h:   newHandler(func(bool, *int, *int) error { return NewError(CodeMethodNotFound, "not found") }),
			in:  "5",
			out: "0",
			err: CodeMethodNotFound,
		},

		{ // 无效的输入 json
			h:   newHandler(func(bool, *int, *int) error { return nil }),
			in:  "{xx",
			out: "0",
			err: CodeParseError,
		},

		{ // 无效的输出 json
			h: newHandler(func(notify bool, in *int, out *float64) error {
				*out = math.NaN()
				return nil
			}),
			in:  "5",
			out: "0",
			err: CodeParseError,
		},

		{ // 无效的输出 json，但是 notify 类型，无视输出，也就无法输出 json 格式错误
			h: newHandler(func(notify bool, in *int, out *float64) error {
				*out = math.NaN()
				return nil
			}),
			in:     "5",
			notify: true,
			out:    "0",
		},
	}

	for i, item := range data {
		in := []byte(item.in)
		req := &body{
			Version: Version,
			ID:      &ID{isNumber: true, number: 1},
			Method:  "f1",
			Params:  (*json.RawMessage)(&in),
		}

		if item.notify {
			req.ID = nil
		}

		resp, err := item.h.call(req)

		switch item.err {
		case 0: // 正常
			a.NotError(err, "not error at %d,%v", i, err)
			if !item.notify {
				a.Equal(string(*resp.Result), item.out, "not equal v1=%v,v2=%v at %d", string(*resp.Result), item.out, i)
			}
		case -1: // 普通错误
			a.Error(err, "err %v at @d", err, i).Nil(resp)
		default:
			err1, ok := err.(*Error)
			a.True(ok).
				Equal(err1.Code, item.err, "not equal v1=%v,v2=%v @ %d", err1.Code, item.err, i).
				Nil(resp)
		}
	}
}
