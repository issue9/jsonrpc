// SPDX-License-Identifier: MIT

package jsonrpc

import (
	"net/http/httptest"
	"testing"

	"github.com/issue9/assert"
)

var _ Transport = &httpTransport{}

var (
	f1 = func(notify bool, params *inType, result *outType) error {
		if notify {
			return nil
		}

		result.Name = params.First + params.Last
		result.Age = params.Age
		return nil
	}
)

func TestConn_ServeHTTP(t *testing.T) {
	a := assert.New(t)
	conn := NewConn(nil)
	a.NotNil(conn)

	a.True(conn.Register("f1", f1))
	a.False(conn.Register("f1", f1))
	a.False(conn.Register("f1", f1))

	srv := httptest.NewServer(conn)
	defer srv.Close()

	client := NewHTTPClient(srv.URL)
	a.NotError(client.Notify("f1", &inType{Age: 18, First: "f", Last: "l"}))
	a.NotError(client.Notify("f2", &inType{})) // notify 不返回错误，即使找不到服务

	out := &outType{}
	a.NotError(client.Request("f1", &inType{Age: 18, First: "f", Last: "l"}, out))
	a.Equal(out.Age, 18).Equal(out.Name, "fl")
	out = &outType{}
	a.Error(client.Request("f2", &inType{Age: 18}, out)) // 不存在的服务名称
	a.Equal(out.Age, 0)
}

func TestValidContentType(t *testing.T) {
	a := assert.New(t)

	a.NotError(validContentType("application/json"))
	a.NotError(validContentType(""))
	a.NotError(validContentType("application/json;charset=utf-8"))
	a.NotError(validContentType("application/json;;charset=utf-8"))
	a.NotError(validContentType("application/json;charset=utf-8"))
	a.NotError(validContentType("application/json;"))

	a.Error(validContentType("text/json;"))
	a.Error(validContentType("application/json;charset="))
	a.Error(validContentType("application/json;charset=utf8"))
}
