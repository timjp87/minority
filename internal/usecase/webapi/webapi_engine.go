package webapi_engine

import (
	"bytes"
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"

	"github.com/ethereum/go-ethereum/log"
	"github.com/karalabe/minority/config"
	"github.com/karalabe/minority/internal/entity"
)

type ClusterWebAPI struct {
	client http.Client
	conf   config.HTTP
}

func New(cfg config.HTTP) *ClusterWebAPI {
	return &ClusterWebAPI{
		client: *http.DefaultClient,
		conf:   cfg,
	}

}

func (w *ClusterWebAPI) Send(msg *entity.JsonRpcRequest) (*entity.JsonRpcResponse, error) {
	// Create new http request
	request := http.Request{
		Header: make(http.Header),
	}
	request.Method = "POST"
	request.Header.Set("Authorization", msg.Header.Get("Authorization"))
	request.URL, _ = url.Parse("http://" + w.conf.String()) // Can't fail as we handle this upstream
	body, err := json.Marshal(msg.Body)
	if err != nil {
		log.Error("Failed to marshal json-rpc request into bytes", "err", err)
		return nil, err
	}
	request.Body = io.NopCloser(bytes.NewReader(body))

	// Send the json-rpc request to the execution client
	response, err := w.client.Do(&request)
	if err != nil {
		log.Error("Failed to send request to execution client", "err", err)
		return nil, err
	}

	// Parse the http response
	var rpcBody entity.JsonRpcMessage
	responseBody, err := ioutil.ReadAll(response.Body)
	if err != nil {
		log.Error("Failed to read execution client response body", "err", err)
		return nil, err
	}
	if err = json.Unmarshal(responseBody, &rpcBody); err != nil {
		log.Error("Failed to unmarshal execution client response body to json-rpc message", "err", err)
		return nil, err
	}

	rpcResponse := &entity.JsonRpcResponse{
		Code: response.StatusCode,
		Body: rpcBody,
	}

	return rpcResponse, nil
}
