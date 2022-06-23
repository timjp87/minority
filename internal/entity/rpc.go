package entity

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const (
	vsn                      = "2.0"
	serviceMethodSeparator   = "_"
	subscribeMethodSuffix    = "_subscribe"
	unsubscribeMethodSuffix  = "_unsubscribe"
	notificationMethodSuffix = "_subscription"

	defaultWriteTimeout = 10 * time.Second // used if context has no deadline
)

var null = json.RawMessage("null")

type subscriptionResult struct {
	ID     string          `json:"subscription"`
	Result json.RawMessage `json:"result,omitempty"`
}

// A value of this type can a JSON-RPC request, notification, successful response or
// error response. Which one it is depends on the fields.
type JsonRpcMessage struct {
	Version string          `json:"JsonRpc,omitempty"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Error   *jsonError      `json:"error,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
}

// Represents the JSON-RPC request with meta data transported over HTTP
type JsonRpcRequest struct {
	Header http.Header
	Body   JsonRpcMessage
}

// Represents the JSON-RPC from a client in response to a request via HTTP
type JsonRpcResponse struct {
	Code   int
	Header http.Header
	Body   JsonRpcMessage
}

func (msg *JsonRpcMessage) isNotification() bool {
	return msg.ID == nil && msg.Method != ""
}

func (msg *JsonRpcMessage) isCall() bool {
	return msg.hasValidID() && msg.Method != ""
}

func (msg *JsonRpcMessage) isResponse() bool {
	return msg.hasValidID() && msg.Method == "" && msg.Params == nil && (msg.Result != nil || msg.Error != nil)
}

func (msg *JsonRpcMessage) hasValidID() bool {
	return len(msg.ID) > 0 && msg.ID[0] != '{' && msg.ID[0] != '['
}

func (msg *JsonRpcMessage) isSubscribe() bool {
	return strings.HasSuffix(msg.Method, subscribeMethodSuffix)
}

func (msg *JsonRpcMessage) isUnsubscribe() bool {
	return strings.HasSuffix(msg.Method, unsubscribeMethodSuffix)
}

func (msg *JsonRpcMessage) namespace() string {
	elem := strings.SplitN(msg.Method, serviceMethodSeparator, 2)
	return elem[0]
}

func (msg *JsonRpcMessage) String() string {
	b, _ := json.Marshal(msg)
	return string(b)
}

type jsonError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func (err *jsonError) Error() string {
	if err.Message == "" {
		return fmt.Sprintf("json-rpc error %d", err.Code)
	}
	return err.Message
}

func (err *jsonError) ErrorCode() int {
	return err.Code
}

func (err *jsonError) ErrorData() interface{} {
	return err.Data
}
