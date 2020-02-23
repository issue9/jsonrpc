// SPDX-License-Identifier: MIT

// Package jsonrpc 实现简单的 JSON RPC 2.0
//
// https://wiki.geekdream.com/Specification/json-rpc_2.0.html
package jsonrpc

import (
	"encoding/json"
	"errors"
)

// Version JSON RPC 的版本
const Version = "2.0"

// JSON RPC 2.0 定义的错误代码
const (
	CodeParseError     = -32700
	CodeInvalidRequest = -32600
	CodeMethodNotFound = -32601
	CodeInvalidParams  = -32602
	CodeInternalError  = -32603
)

// 一些错误定义
var (
	ErrInvalidHeader      = errors.New("无效的报头格式")
	ErrInvalidContentType = errors.New("无效的报头 Content-Type")
	ErrMissContentLength  = errors.New("缺少 Content-Length 报头")
	ErrIDNotEqual         = NewError(CodeInvalidParams, "ID 不相等")
	ErrInvalidRequest     = NewError(CodeInvalidRequest, "无效的请求内容")
	ErrMethodNotFound     = NewError(CodeMethodNotFound, "未找到对应的服务实现")
)

// Error JSON-RPC 返回的错误类型
type Error struct {
	// 错误代码
	Code int `json:"code"`

	// 错误的简短描述
	Message string `json:"message"`

	// 详细的错误描述信息，可以为空
	Data interface{} `json:"data,omitempty"`
}

// ID 用于表示唯一的请求 ID，可以是数值，字符串
type ID struct {
	number   int64
	alpha    string
	isNumber bool
}

// Equal 两个 ID 是否相等
func (id *ID) Equal(val *ID) bool {
	if id.isNumber != val.isNumber {
		return false
	}

	if id.isNumber {
		return id.number == val.number
	}
	return id.alpha == val.alpha
}

func (id *ID) MarshalJSON() ([]byte, error) {
	if id.isNumber {
		return json.Marshal(id.number)
	}
	return json.Marshal(id.alpha)
}

func (id *ID) UnmarshalJSON(data []byte) error {
	if err := json.Unmarshal(data, &id.number); err == nil {
		id.isNumber = true
		return nil
	}

	id.isNumber = false
	return json.Unmarshal(data, &id.alpha)
}

// Transport 用于操作 JSON RPC 的传输层接口
type Transport interface {
	// 从转输层读取内容并转换成对象 v
	//
	// 如果返回的是 *Error 类型的错误，则直接将该错误信息反馈给客户端，
	// 如果是普通错误，则统一转换成 CodeParseError 传递给客户端。
	Read(v interface{}) error

	// 将对象 v 写入传输层
	//
	// 如果返回的是 *Error 类型的错误，则直接将该错误信息反馈给客户端，
	// 如果是普通错误，则错误代码不确定。
	Write(v interface{}) error

	Close() error
}

type request struct {
	// 指定 JSON-RPC 协议版本的字符串
	Version string `json:"jsonrpc"`

	// 已建立客户端的唯一标识 id，值必须包含一个字符串、数值或 NULL 空值。
	// 如果不包含该成员则被认定为是一个通知。该值一般不为 NULL，若为数值则不应该包含小数。
	ID *ID `json:"id,omitempty"`

	// 包含所要调用方法名称的字符串
	//
	// 以 rpc 开头的方法名，用英文句号（U+002E or ASCII 46）
	// 连接的为预留给 rpc 内部的方法名及扩展名，且不能在其他地方使用。
	Method string `json:"method"`

	// 调用方法所需要的结构化参数值，该成员参数可以被省略。
	Params *json.RawMessage `json:"params,omitempty"`
}

type response struct {
	// 指定 JSON-RPC 协议版本的字符串
	Version string `json:"jsonrpc"`

	// 成功时的返回结果，如果不成功，则不应该输出该对象。
	Result *json.RawMessage `json:"result,omitempty"`

	// 失败时的返回结果，如果成功，则不应该输出该对象。
	Error *Error `json:"error,omitempty"`

	// ID 返回请求端的 ID，如果检查 ID 失败时，返回空值
	ID *ID `json:"id,omitempty"`
}

func (req *request) isEmpty() bool {
	return req.Version == "" && req.ID == nil && req.Method == "" && req.Params == nil
}

// NewError 新的 Error 对象
func NewError(code int, msg string) *Error {
	return NewErrorWithData(code, msg, nil)
}

// NewErrorWithData 新的 Error 对象
func NewErrorWithData(code int, msg string, data interface{}) *Error {
	return &Error{
		Code:    code,
		Message: msg,
		Data:    data,
	}
}

func NewErrorWithError(code int, err error) *Error {
	if err2, ok := err.(*Error); ok {
		return err2
	}

	return NewError(code, err.Error())
}

func (err *Error) Error() string {
	return err.Message
}
