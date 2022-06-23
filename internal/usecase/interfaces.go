package usecase

import "github.com/karalabe/minority/internal/entity"

type (
	Cluster interface {
		MaintainTopology()
		HandleConsensusRequest(*entity.JsonRpcRequest) (*entity.JsonRpcResponse, error)
		ForwardConsensusRequest() error
	}
	ClusterBroker interface {
		PublishRequest(*entity.JsonRpcRequest) error
		PublishResponse(entity.Topic, *entity.JsonRpcResponse) error
		ConsumeUpdate(*entity.Update)
		ConsumeRequest(*entity.JsonRpcRequest)
		ConsumeResponse(*entity.JsonRpcResponse)
		NotifyUpdate() <-chan *entity.Update
		NotifyResponse() <-chan *entity.JsonRpcResponse
		NotifyRequest() <-chan *entity.JsonRpcRequest
	}
	ClusterWebAPI interface {
		Send(*entity.JsonRpcRequest) (*entity.JsonRpcResponse, error)
	}
)
