package usecase

import (
	"github.com/ethereum/go-ethereum/log"
	"github.com/karalabe/minority/internal/entity"
)

type ClusterUseCase struct {
	Mode    entity.RelayMode
	cluster *entity.Cluster
	Broker  ClusterBroker
	webapi  ClusterWebAPI
}

func New(mode entity.RelayMode, c *entity.Cluster, b ClusterBroker, w ClusterWebAPI) *ClusterUseCase {
	return &ClusterUseCase{
		Mode:    mode,
		cluster: c,
		Broker:  b,
		webapi:  w,
	}
}

func (uc *ClusterUseCase) HandleConsensusRequest(req *entity.JsonRpcRequest) (*entity.JsonRpcResponse, error) {
	log.Info("Handeling json_rpc request...", "id", string(req.Body.ID), "version", req.Body.Version, "method", req.Body.Method)

	if err := uc.Broker.PublishRequest(req); err != nil {
		log.Error("Failed to publish request", "err", err)
		return nil, err
	}

	resp := <-uc.Broker.NotifyResponse()
	return resp, nil
}

func (uc *ClusterUseCase) ForwardConsensusRequest() error {
	req := <-uc.Broker.NotifyRequest()
	log.Info("Forwarding json rpc message", "id", req.Body.ID, "version", req.Body.Version, "method", req.Body.Method)

	resp, err := uc.webapi.Send(req)
	if err != nil {
		log.Error("Failed to send request to execution client", "err", err)
		return err
	}

	if resp.Code != 200 {
		log.Error("Execution engine responded with error", "err", resp.Body.Error)
		if err := uc.Broker.PublishResponse(entity.Topic(req.Body.Method)+"_resp", resp); err != nil {
			log.Error("Failed to publish response", "err", err)
			return err
		}
		return nil
	} else {
		log.Info("Publishing execution engine response", "id", req.Body.ID, "version", req.Body.Version, "method", req.Body.Method)
		if err := uc.Broker.PublishResponse(entity.Topic(req.Body.Method)+"_resp", resp); err != nil {
			log.Error("Failed to publish response", "err", err)
			return err
		}
		return nil
	}
}

func (uc *ClusterUseCase) MaintainTopology() {}
