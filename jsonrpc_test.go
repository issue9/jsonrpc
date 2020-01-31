// SPDX-License-Identifier: MIT

package jsonrpc

import (
	"encoding/json"
	"testing"

	"github.com/issue9/assert"
)

var (
	_ error = &Error{}

	_ json.Marshaler   = &requestID{}
	_ json.Unmarshaler = &requestID{}
)

func TestRequestID_equal(t *testing.T) {
	a := assert.New(t)

	v1 := &requestID{isNumber: true}
	v2 := &requestID{isNumber: false}
	a.False(v1.equal(v2))

	v1 = &requestID{isNumber: true, number: 1}
	v2 = &requestID{isNumber: true, number: 1, alpha: "11"}
	a.True(v1.equal(v2))

	v1 = &requestID{isNumber: true, number: 12}
	v2 = &requestID{isNumber: true, number: 1, alpha: "11"}
	a.False(v1.equal(v2))

	v1 = &requestID{isNumber: false, number: 1, alpha: "11"}
	v2 = &requestID{isNumber: false, number: 1, alpha: "11"}
	a.True(v1.equal(v2))

	v1 = &requestID{isNumber: false, number: 1, alpha: "112"}
	v2 = &requestID{isNumber: false, number: 1, alpha: "11"}
	a.False(v1.equal(v2))
}

func TestRequestID_MarshalJSON(t *testing.T) {
	a := assert.New(t)

	var id *requestID
	data, err := json.Marshal(id)
	a.NotError(err).
		Equal(string(data), "null")

	id = &requestID{
		isNumber: true,
		number:   0,
	}
	data, err = json.Marshal(id)
	a.NotError(err).Equal(string(data), "0")

	id = &requestID{
		isNumber: false,
		number:   11,
		alpha:    "11",
	}
	data, err = json.Marshal(id)
	a.NotError(err).Equal(string(data), "\"11\"")
}

func TestRequestID_UnmarshalJSON(t *testing.T) {
	a := assert.New(t)

	var id = &requestID{}
	a.NotError(json.Unmarshal([]byte("0"), id))
	a.True(id.isNumber).
		Equal(id.number, 0).
		Empty(id.alpha)

	id = &requestID{}
	a.NotError(json.Unmarshal([]byte("1"), id))
	a.True(id.isNumber).
		Equal(id.number, 1).
		Empty(id.alpha)

	id = &requestID{}
	a.NotError(json.Unmarshal([]byte("\"1\""), id))
	a.False(id.isNumber).
		Equal(id.number, 0).
		Equal(id.alpha, "1")

	id = &requestID{}
	a.NotError(json.Unmarshal([]byte("\"\""), id))
	a.False(id.isNumber).
		Equal(id.number, 0).
		Empty(id.alpha)

	req := &request{}
	a.NotError(json.Unmarshal([]byte(`{"id":0}`), req))
	a.Equal(req.ID.number, 0).True(req.ID.isNumber)

	req = &request{}
	a.NotError(json.Unmarshal([]byte(`{}`), req))
	a.Nil(req.ID)
}
