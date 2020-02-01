// SPDX-License-Identifier: MIT

package jsonrpc

import (
	"errors"
	"log"
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

func initConn(a *assert.Assertion, errlog *log.Logger) *Conn {
	conn := NewConn(errlog)
	a.NotNil(conn)

	a.True(conn.Register("f1", f1))
	a.True(conn.Register("f2", f2))
	a.True(conn.Register("f3", f3))

	a.False(conn.Register("f3", f3))

	return conn
}

func TestConn_Registers(t *testing.T) {
	a := assert.New(t)
	conn := NewConn(nil)
	a.NotError(conn)

	a.NotPanic(func() {
		conn.Registers(map[string]interface{}{
			"f1": f1,
			"f2": f2,
		})
	})

	conn = NewConn(nil)
	a.NotError(conn)
	a.Panic(func() {
		conn.Registers(map[string]interface{}{
			"f1": f1,
			"f2": initConn, // 签名不正确
		})
	})

	conn = NewConn(nil)
	a.NotError(conn)
	a.Panic(func() {
		a.True(conn.Register("f1", f1))
		conn.Registers(map[string]interface{}{
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
