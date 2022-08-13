// SPDX-License-Identifier: MIT

package jsonrpc

import (
	"encoding/json"
	"testing"

	"github.com/issue9/assert/v3"
)

var (
	_ error = &Error{}

	_ json.Marshaler   = &ID{}
	_ json.Unmarshaler = &ID{}
)

func TestID_Equal(t *testing.T) {
	a := assert.New(t, false)

	v1 := &ID{isNumber: true}
	v2 := &ID{isNumber: false}
	a.False(v1.Equal(v2))

	v1 = &ID{isNumber: true, number: 1}
	v2 = &ID{isNumber: true, number: 1, alpha: "11"}
	a.True(v1.Equal(v2))

	v1 = &ID{isNumber: true, number: 12}
	v2 = &ID{isNumber: true, number: 1, alpha: "11"}
	a.False(v1.Equal(v2))

	v1 = &ID{isNumber: false, number: 1, alpha: "11"}
	v2 = &ID{isNumber: false, number: 1, alpha: "11"}
	a.True(v1.Equal(v2))

	v1 = &ID{isNumber: false, number: 1, alpha: "112"}
	v2 = &ID{isNumber: false, number: 1, alpha: "11"}
	a.False(v1.Equal(v2))
}

func TestID_MarshalJSON(t *testing.T) {
	a := assert.New(t, false)

	var id *ID
	data, err := json.Marshal(id)
	a.NotError(err).
		Equal(string(data), "null")

	id = &ID{
		isNumber: true,
		number:   0,
	}
	data, err = json.Marshal(id)
	a.NotError(err).Equal(string(data), "0")

	id = &ID{
		isNumber: false,
		number:   11,
		alpha:    "11",
	}
	data, err = json.Marshal(id)
	a.NotError(err).Equal(string(data), "\"11\"")
}

func TestID_UnmarshalJSON(t *testing.T) {
	a := assert.New(t, false)

	var id = &ID{}
	a.NotError(json.Unmarshal([]byte("0"), id))
	a.True(id.isNumber).
		Equal(id.number, 0).
		Empty(id.alpha)

	id = &ID{}
	a.NotError(json.Unmarshal([]byte("1"), id))
	a.True(id.isNumber).
		Equal(id.number, 1).
		Empty(id.alpha)

	id = &ID{}
	a.NotError(json.Unmarshal([]byte("\"1\""), id))
	a.False(id.isNumber).
		Equal(id.number, 0).
		Equal(id.alpha, "1")

	id = &ID{}
	a.NotError(json.Unmarshal([]byte("\"\""), id))
	a.False(id.isNumber).
		Equal(id.number, 0).
		Empty(id.alpha)

	req := &body{}
	a.NotError(json.Unmarshal([]byte(`{"id":0}`), req))
	a.Equal(req.ID.number, 0).True(req.ID.isNumber)

	req = &body{}
	a.NotError(json.Unmarshal([]byte(`{}`), req))
	a.Nil(req.ID)
}

func TestID_String(t *testing.T) {
	a := assert.New(t, false)

	id := &ID{alpha: "123"}
	a.Equal(id.String(), "123")

	id.isNumber = true
	a.Equal(id.String(), "0")

	id.number = -133
	a.Equal(id.String(), "-133")
}
