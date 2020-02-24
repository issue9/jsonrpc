// SPDX-License-Identifier: MIT

package jsonrpc

import (
	"bytes"
	"math"
	"testing"

	"github.com/issue9/assert"
)

var _ Transport = &streamTransport{}

func TestStreamTransport_Read(t *testing.T) {
	a := assert.New(t)

	data := []*struct {
		header bool // 是否带报头
		in     string
		req    *body
		err    bool
	}{
		{
			err: true,
		},
		{
			in:  `{}`,
			req: &body{},
		},
		{ // 没有报头，出错
			header: true,
			in:     `{}`,
			req:    &body{},
			err:    true,
		},

		{ // 没有报头，出错
			in:  `{"jsonrpc":"2.0","id":"1"}`,
			req: &body{Version: Version, ID: &ID{alpha: "1"}},
		},
		{
			header: true,
			in:     `{"jsonrpc":"2.0","id":"1"}`,
			req:    &body{Version: Version, ID: &ID{alpha: "1"}},
			err:    true,
		},

		{
			in:  `}`,
			req: &body{},
			err: true,
		},
		{
			header: true,
			in:     `}`,
			req:    &body{},
			err:    true,
		},

		{
			header: true,
			in:     "Content-Length:2\r\n\r\n{}",
			req:    &body{},
		},
		{
			header: true,
			in:     "Content-Type: application/json-rpc;charset=utf-8\r\nContent-Length:3\r\n\r\n{ }",
			req:    &body{},
		},
		{ // 包含非标准报头
			header: true,
			in:     "User-Agent:go\r\nContent-Type: application/json-rpc;charset=utf-8\r\nContent-Length:3\r\n\r\n{ }",
			req:    &body{},
		},
		{
			header: true,
			in:     "Content-Type: application/json-rpc;charset=utf-8\r\nContent-Length:17\r\n\r\n{\"jsonrpc\":\"2.0\"}",
			req:    &body{Version: Version},
		},

		{ // 长度不正确
			header: true,
			in:     "Content-Length:2\r\n\r\n{ }",
			req:    &body{},
			err:    true,
		},
		{ // 长度为无效的数值
			header: true,
			in:     "Content-Length:NaN\r\n\r\n{ }",
			req:    &body{},
			err:    true,
		},
		{ // 未指定长度
			header: true,
			in:     "{}",
			req:    &body{},
			err:    true,
		},
		{ // 报头格式不正确
			header: true,
			in:     "Content-Type-xx\r\n\r\n{}",
			req:    &body{},
			err:    true,
		},
		{ // 无效的 content-type
			header: true,
			in:     "Content-Type:text/xml\r\n\r\n{}",
			req:    &body{},
			err:    true,
		},
		{ // 无效的 content-type
			header: true,
			in:     "Content-Type:application/jsonrequest;charset=gbk\r\n\r\n{}",
			req:    &body{},
			err:    true,
		},
		{ // 无效的 content-length
			header: true,
			in:     "Content-length:-1\r\n\r\n{}",
			req:    &body{},
			err:    true,
		},
		{ // 未指定 content-length，无法读取内容
			header: true,
			in:     "Content-Type:application/json\r\n\r\n{\"jsonrpc\":\"2.0\"}",
			req:    &body{},
		},
	}

	for i, item := range data {
		in, out := bytes.NewBufferString(item.in), new(bytes.Buffer)
		transport := NewStreamTransport(item.header, in, out, nil)
		a.NotError(transport)

		req := &body{}
		err := transport.Read(req)

		if item.err {
			a.Error(err, "not error @ %d", i)
			continue
		}

		a.NotError(err, "error %s @ %d", err, i)
		a.Equal(req, item.req, "not equal @ %d", i)
	}
}

func TestStreamTransport_Write(t *testing.T) {
	a := assert.New(t)

	data := []*struct {
		header bool
		resp   *body
		out    string
		err    bool
	}{
		{
			out: "null",
		},
		{
			header: true,
			out:    "Content-Type: application/json;charset=utf-8\r\nContent-Length: 4\r\n\r\nnull",
		},

		{
			resp: &body{},
			out:  `{"jsonrpc":""}`, // jsonrpc 这个字段是非缺省字段
		},
		{
			header: true,
			resp:   &body{},
			out:    "Content-Type: application/json;charset=utf-8\r\nContent-Length: 14\r\n\r\n{\"jsonrpc\":\"\"}", // jsonrpc 这个字段是非缺省字段
		},

		{
			header: true,
			resp:   &body{ID: &ID{isNumber: true, number: 22}},
			out:    "Content-Type: application/json;charset=utf-8\r\nContent-Length: 22\r\n\r\n{\"jsonrpc\":\"\",\"id\":22}", // jsonrpc 这个字段是非缺省字段
		},
	}

	for i, item := range data {
		in, out := new(bytes.Buffer), new(bytes.Buffer)
		transport := NewStreamTransport(item.header, in, out, nil)
		a.NotNil(transport)

		err := transport.Write(item.resp)
		if item.err {
			a.Error(err, "not err at %d", i)
			continue
		}

		a.NotError(err, "err %v @ %d", err, i)
		a.Equal(out.String(), item.out, "not equal v1=%s,v2=%s, at %d", out.String(), item.out, i)
	}

	// 非正确输出
	type failedTester struct {
		Value float64
	}
	in, out := new(bytes.Buffer), new(bytes.Buffer)
	transport := NewStreamTransport(true, in, out, nil)
	a.NotNil(transport)
	a.Error(transport.Write(&failedTester{Value: math.NaN()}))
	a.NotError(transport.Close())
}
