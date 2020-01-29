// SPDX-License-Identifier: MIT

package jsonrpc

import (
	"testing"

	"github.com/issue9/assert"
)

var _ Transport = &httpTransport{}

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
