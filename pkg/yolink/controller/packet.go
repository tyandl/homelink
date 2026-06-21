package controller

import (
	"encoding/json"
	"time"

	"github.com/tyandl/homelink/pkg/yolink/types"
)

// basicDownloadDataPacket (BDDP) is the request envelope sent to the YoLink API. Its fields
// stay exported for JSON marshaling; only the type is package-internal.
type basicDownloadDataPacket struct {
	Time         types.Timestamp `json:"time"`
	Method       string          `json:"method"`
	MsgId        string          `json:"msgid,omitempty"`
	TargetDevice string          `json:"targetDevice,omitempty"`
	Token        string          `json:"token,omitempty"`
	Params       any             `json:"params,omitempty"`
}

// newBDDP builds a request packet. targetDevice and token are nil for home-level calls and
// set for device-addressed calls.
func newBDDP(method string, params any, targetDevice *string, token *string) *basicDownloadDataPacket {
	packet := &basicDownloadDataPacket{Time: types.Timestamp{Time: time.Now().UTC()}, Method: method, Params: params}
	if targetDevice != nil {
		packet.TargetDevice = *targetDevice
	}
	if token != nil {
		packet.Token = *token
	}
	return packet
}

// basicUploadDataPacket (BUDP) is the response envelope returned by the YoLink API.
type basicUploadDataPacket struct {
	Time   types.Timestamp  `json:"time"`
	Method string           `json:"method"`
	MsgId  interface{}      `json:"msgid"`
	Code   types.StatusCode `json:"code"`
	Desc   string           `json:"desc,omitempty"`
	Data   json.RawMessage  `json:"data,omitempty"`
}
