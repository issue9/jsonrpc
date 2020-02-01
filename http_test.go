// SPDX-License-Identifier: MIT

package jsonrpc

import (
	"net/http/httptest"
	"testing"

	"github.com/issue9/assert"
)

var _ Transport = &httpTransport{}

func TestConn_ServeHTTP(t *testing.T) {
	a := assert.New(t)
	conn := initConn(a, nil)

	srv := httptest.NewServer(conn)
	defer srv.Close()

	client := NewHTTPClient(srv.URL)
	a.NotError(client.Notify("f1", &inType{Age: 18, First: "f", Last: "l"}))
	a.NotError(client.Notify("not-found", &inType{})) // notify 不返回错误，即使找不到服务

	out := &outType{}
	a.NotError(client.Send("f1", &inType{Age: 18, First: "f", Last: "l"}, out))
	a.Equal(out.Age, 18).Equal(out.Name, "fl")

	// 检测抛出错误是否正确
	out = &outType{}
	err := client.Send("f2", &inType{Age: 18}, out)
	err1, ok := err.(*Error)
	a.True(ok).Equal(err1.Code, CodeInvalidParams) // 由函数 f2 抛出的错误 Error

	// 检测抛出错误是否正确
	out = &outType{}
	err = client.Send("f3", &inType{Age: 18}, out)
	err1, ok = err.(*Error)
	a.True(ok).Equal(err1.Code, CodeInternalError) // 由函数 f3 抛出的普通错误

	out = &outType{}
	a.Error(client.Send("not-found", &inType{Age: 18}, out)) // 不存在的服务名称
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
