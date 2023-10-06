package types

import (
	"encoding/json"
	"errors"
	"google.golang.org/grpc/codes"
)

type CallRequest struct {
	//对端id
	PeerId string `json:"peerId"`
	//请求id
	CallId string `json:"callId"`
	//请求方法
	Method string `json:"method"`
	//请求携带的数据
	Data []byte `json:"data"`
}

var (
	ErrInvalidPeerId = errors.New("invalid peer id")
	ErrInvalidCallId = errors.New("invalid call id")
	ErrInvalidMethod = errors.New("invalid method")
)

func (r *CallRequest) FromBytes(data []byte) error {
	return json.Unmarshal(data, r)
}

func (r *CallRequest) Validate() error {
	if r.PeerId == "" {
		return ErrInvalidPeerId
	}
	if r.CallId == "" {
		return ErrInvalidCallId
	}
	if r.Method == "" {
		return ErrInvalidMethod
	}
	if r.Data == nil {
		r.Data = []byte{}
	}
	return nil
}

func (r *CallRequest) ToBytes() []byte {
	marshal, _ := json.Marshal(r)
	return marshal
}

type CallResponse struct {
	//请求id
	CallId string `json:"callId"`
	//请求方法
	Method string `json:"method"`
	//响应状态
	Status codes.Code `json:"status"`
	//响应携带的数据
	Data []byte `json:"data"`
}

func (r *CallResponse) ToBytes() []byte {
	marshal, _ := json.Marshal(r)
	return marshal
}

func (r *CallResponse) FromBytes(msg []byte) error {
	return json.Unmarshal(msg, r)
}

var (
	RequestUnmarshalErrorResponse = &CallResponse{
		CallId: "",
		Method: "",
		Status: codes.InvalidArgument,
		Data:   nil,
	}
	PeerOfflineResponseError = errors.New("peer offline")
	CallTimeoutResponseError = errors.New("call timeout")
)

func PeerOfflineResponse(callId string, method string) *CallResponse {
	return &CallResponse{
		CallId: callId,
		Method: method,
		Status: codes.Unavailable,
		Data:   nil,
	}
}

func CallTimeoutResponse(callId string, method string) *CallResponse {
	return &CallResponse{
		CallId: callId,
		Method: method,
		Status: codes.DeadlineExceeded,
		Data:   nil,
	}
}
