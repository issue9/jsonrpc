// SPDX-License-Identifier: MIT

package jsonrpc

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/issue9/autoinc"
)

// 一些错误定义
var (
	ErrInvalidContentType = errors.New("无效的报头 Content-Type")
	ErrMissContentLength  = errors.New("缺少 Content-Length 报头")
)

const (
	contentType   = "content-Type"
	contentLength = "content-length"
	charset       = "utf-8"
	mimetype      = "application/json"
)

// HTTPClient http 的客户端
type HTTPClient struct {
	autoinc *autoinc.AutoInc
	path    string
}

type httpTransport struct {
	r    *http.Request
	w    http.ResponseWriter
	wMux sync.Mutex
}

// NewHTTPClient 声明新的 HTTPClient 对象
func NewHTTPClient(path string) *HTTPClient {
	return &HTTPClient{
		autoinc: autoinc.New(0, 1, 100),
		path:    path,
	}
}

// Notify 请求 JSON RPC 服务端
func (client *HTTPClient) Notify(method string, params interface{}) error {
	return client.request(method, true, params, nil)
}

// Request 请求 JSON RPC 服务端
func (client *HTTPClient) Request(method string, params, result interface{}) error {
	return client.request(method, false, params, result)
}

func (client *HTTPClient) request(method string, notify bool, params, result interface{}) error {
	data, err := json.Marshal(params)
	if err != nil {
		return err
	}

	req := &request{
		Version: Version,
		Method:  method,
		Params:  (*json.RawMessage)(&data),
	}
	if !notify {
		req.ID = &requestID{isNumber: true, number: client.autoinc.MustID()}
	}

	body, err := json.Marshal(req)
	if err != nil {
		return err
	}

	resp, err := http.Post(client.path, mimetype, bytes.NewBuffer(body))
	if err != nil {
		return err
	}

	if notify {
		return nil
	}

	data, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	r := &response{}
	if err = json.Unmarshal(data, r); err != nil {
		return err
	}

	if r.Error != nil {
		return r.Error
	}
	return json.Unmarshal(*r.Result, result)
}

// NewHTTPTransport 声明基于 HTTP 的 Transport 实例
//
// https://www.simple-is-better.org/json-rpc/transport_http.html
func NewHTTPTransport(w http.ResponseWriter, r *http.Request) Transport {
	return &httpTransport{
		r: r,
		w: w,
	}
}

// Read 读取内容，先验证报头，并返回实际 body 的内容
func (s *httpTransport) Read(v interface{}) error {
	if err := validContentType(s.r.Header.Get(contentType)); err != nil {
		return err
	}

	cl := s.r.Header.Get(contentLength)
	if cl == "" {
		return ErrMissContentLength
	}
	l, err := strconv.ParseInt(cl, 10, 64)
	if err != nil {
		return err
	}

	data := make([]byte, l)
	n, err := io.ReadFull(s.r.Body, data)
	if err != nil {
		return err
	}

	return json.Unmarshal(data[:n], v)
}

func (s *httpTransport) Write(obj interface{}) error {
	data, err := json.Marshal(obj)
	if err != nil {
		return err
	}

	s.wMux.Lock()
	defer s.wMux.Unlock()

	s.w.Header().Set(contentType, mimetype)
	s.w.Header().Set(contentLength, strconv.Itoa(len(data)))
	_, err = s.w.Write(data)
	return err
}

// 验证 content-type 的正确性
//
// 如果存在该值，则必须要以 mimetype 开头，
// charset 如果有指定，必须为 utf-8，否则不作判断
func validContentType(header string) error {
	if header == "" {
		return nil
	}

	pairs := strings.Split(header, ";")

	if strings.ToLower(pairs[0]) != mimetype {
		return ErrInvalidContentType
	}

	for _, pair := range pairs[1:] {
		index := strings.IndexByte(pair, '=')
		if index > 0 &&
			strings.ToLower(strings.TrimSpace(pair[:index])) == "charset" &&
			strings.ToLower(strings.TrimSpace(pair[index+1:])) != charset {
			return ErrInvalidContentType
		}
	}

	return nil
}
