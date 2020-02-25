// SPDX-License-Identifier: MIT

package jsonrpc

import (
	"encoding/json"
	"fmt"
	"reflect"
)

var errType = reflect.TypeOf((*error)(nil)).Elem()

type handler struct {
	f       reflect.Value
	in, out reflect.Type
}

// Send 的处理函数
type callback struct {
	f      reflect.Value
	result reflect.Type
}

func newCallback(f interface{}) *callback {
	t := reflect.TypeOf(f)

	if t.Kind() != reflect.Func ||
		t.NumIn() != 1 ||
		t.In(0).Kind() != reflect.Ptr ||
		!t.Out(0).Implements(errType) {
		panic(fmt.Sprintf("函数 %s 签名不正确", t.String()))
	}

	in := t.In(0).Elem()
	if in.Kind() == reflect.Func || in.Kind() == reflect.Ptr || in.Kind() == reflect.Invalid {
		panic(fmt.Sprintf("函数 %s 签名不正确", t.String()))
	}

	return &callback{
		f:      reflect.ValueOf(f),
		result: in,
	}
}

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

func (h *handler) call(req *body) (*body, error) {
	inValue := reflect.New(h.in)
	if req.Params != nil {
		if err := json.Unmarshal(*req.Params, inValue.Interface()); err != nil {
			return nil, NewErrorWithError(CodeParseError, err)
		}
	}

	notify := req.ID == nil
	outValue := reflect.New(h.out)
	ret := h.f.Call([]reflect.Value{reflect.ValueOf(notify), inValue, outValue})
	if !ret[0].IsNil() {
		return nil, NewErrorWithError(CodeInternalError, ret[0].Interface().(error))
	}

	if notify {
		return nil, nil
	}

	data, err := json.Marshal(outValue.Interface())
	if err != nil {
		return nil, NewErrorWithError(CodeParseError, err)
	}

	return &body{
		Version: Version,
		Result:  (*json.RawMessage)(&data),
		ID:      req.ID,
	}, nil
}

func (c *callback) call(body *body) error {
	if body.Error != nil {
		return body.Error
	}

	rv := reflect.New(c.result)
	if body.Result != nil {
		if err := json.Unmarshal(*body.Result, rv.Interface()); err != nil {
			return err
		}
	}

	ret := c.f.Call([]reflect.Value{rv})
	if !ret[0].IsNil() {
		return ret[0].Interface().(error)
	}
	return nil
}
