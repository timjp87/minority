package controller

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/ethereum/go-ethereum/log"
	"github.com/julienschmidt/httprouter"
	"github.com/karalabe/minority/internal/entity"
	"github.com/karalabe/minority/internal/usecase"
)

func NewRouter(router *httprouter.Router, uc usecase.Cluster) *httprouter.Router {
	router.POST("/", func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		msgBody, _ := io.ReadAll(r.Body)
		var rpcMsg entity.JsonRpcMessage
		if err := json.Unmarshal(msgBody, &rpcMsg); err != nil {
			log.Error("Failed to unmarshal consensus request to json-rpc message", "err", err)
		}
		rpcReq := &entity.JsonRpcRequest{
			Header: r.Header,
			Body:   rpcMsg,
		}
		rpcResp, err := uc.HandleConsensusRequest(rpcReq)
		if err != nil {
			log.Error("Failed to handel consensus request", "err", err)
		}

		// Responde with the response we received from execution client side
		w.Header().Set("Authorization", rpcResp.Header.Get("Authorization"))
		w.Header().Set("Content-Type", rpcResp.Header.Get("Content-Type"))
		w.WriteHeader(rpcResp.Code)
		json.NewEncoder(w).Encode(rpcResp.Body)
	})

	return router
}
