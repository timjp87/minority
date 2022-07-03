package broker

import (
	"encoding/json"
	"time"

	"github.com/ethereum/go-ethereum/log"
	"github.com/karalabe/minority/internal/entity"
	"github.com/karalabe/minority/pkg/crypto"
	nsqlogger "github.com/karalabe/minority/pkg/logger"
	"github.com/karalabe/minority/pkg/nsqd"
	"github.com/nsqio/go-nsq"
	NSQ "github.com/nsqio/nsq/nsqd"
)

type ClusterBroker struct {
	Name       string
	cluster    *entity.Cluster
	daemon     *NSQ.NSQD
	tlsCert    []byte // Certificate to use for authenticating to other brokers
	tlsKey     []byte // Private key to use for encrypting traffic with other brokers
	Producer   *nsq.Producer
	Consumers  map[entity.Topic]*nsq.Consumer
	UpdateChan chan *entity.Update
	RespChan   chan *entity.JsonRpcResponse
	ReqChan    chan *entity.JsonRpcRequest
}

func New(nsqd *nsqd.NSQD, mode entity.RelayMode, c *entity.Cluster, secret string) (*ClusterBroker, error) {
	tlsCert, tlsKey := crypto.MakeTLSCert(secret)

	cb := &ClusterBroker{
		Name:       nsqd.Name,
		cluster:    c,
		daemon:     nsqd.Daemon,
		tlsCert:    tlsCert,
		tlsKey:     tlsKey,
		RespChan:   make(chan *entity.JsonRpcResponse, 1),
		ReqChan:    make(chan *entity.JsonRpcRequest, 1),
		UpdateChan: make(chan *entity.Update),
		Consumers:  make(map[entity.Topic]*nsq.Consumer),
	}

	config := nsq.NewConfig()
	config.Snappy = true
	config.TlsV1 = true
	config.TlsConfig = crypto.MakeTLSConfig(tlsCert, tlsKey)

	producer, err := nsq.NewProducer(cb.daemon.RealTCPAddr().String(), config)
	producer.SetLogger(&nsqlogger.NSQProducerLogger{Logger: log.New()}, nsq.LogLevelInfo)

	cb.Producer = producer

	if err != nil {
		return nil, err
	}

	rpcMsg := entity.JsonRpcMessage{
		Method: "announce",
	}

	announcementResp := entity.JsonRpcResponse{
		Code: 200,
		Body: rpcMsg,
	}

	announcementReq := entity.JsonRpcRequest{
		Body: rpcMsg,
	}

	for _, topic := range entity.EthereumTopics {

		var consumer *nsq.Consumer

		switch mode {
		case entity.Consensus:
			// Publish an announcement message to precreate the topic.
			// This seems a bit stupid but somehow it's needed.

			clusterMessage := &entity.Message{
				MessageType: entity.ResponseMessage,
				Response:    &announcementResp,
			}

			bytes, _ := json.Marshal(clusterMessage)
			if err := producer.Publish(string(topic)+"_resp", bytes); err != nil {
				return nil, err
			}

			consumer, err = nsq.NewConsumer(string(topic)+"_resp", nsqd.Name, config)
			consumer.SetLogger(&nsqlogger.NSQConsumerLogger{Logger: log.New()}, nsq.LogLevelInfo)

			if err != nil {
				return nil, err
			}

			cb.Consumers[topic+"_resp"] = consumer
		case entity.Execution:
			// Publish an announcement message to precreate the topic

			clusterMessage := &entity.Message{
				MessageType: entity.RequestMessage,
				Request:     &announcementReq,
			}
			bytes, _ := json.Marshal(clusterMessage)
			if err := producer.Publish(string(topic)+"_req", bytes); err != nil {
				return nil, err
			}

			consumer, err = nsq.NewConsumer(string(topic)+"_req", nsqd.Name, config)
			consumer.SetLogger(&nsqlogger.NSQConsumerLogger{Logger: log.New()}, nsq.LogLevelInfo)

			if err != nil {
				return nil, err
			}

			cb.Consumers[topic+"_req"] = consumer
		}

	}

	// Publish an announcement update message to precreate the topic
	update := &entity.Update{
		Mode:  mode,
		Owner: cb.Name,
		Time:  uint64(time.Now().UnixNano()),
		Nodes: cb.cluster.Views[cb.Name],
	}

	clusterMessage := &entity.Message{
		MessageType: entity.UpdateMessage,
		Update:      update,
	}
	updateMsg, _ := json.Marshal(clusterMessage)
	if err := producer.Publish(string(entity.TopologyTopic), updateMsg); err != nil {
		return nil, err
	}
	consumer, err := nsq.NewConsumer(string(entity.TopologyTopic), nsqd.Name, config)
	consumer.SetLogger(&nsqlogger.NSQConsumerLogger{Logger: log.New()}, nsq.LogLevelInfo)

	if err != nil {
		return nil, err
	}

	cb.Consumers[entity.TopologyTopic] = consumer

	return cb, nil
}

func (b *ClusterBroker) PublishRequest(msg *entity.JsonRpcRequest) error {
	clusterMessage := &entity.Message{
		MessageType: entity.RequestMessage,
		Request:     msg,
	}
	msgBytes, err := json.Marshal(clusterMessage)
	if err != nil {
		return err
	}
	if err := b.Producer.Publish(msg.Body.Method+"_req", msgBytes); err != nil {
		return err
	}
	return nil
}

func (b *ClusterBroker) PublishResponse(topic entity.Topic, msg *entity.JsonRpcResponse) error {
	clusterMessage := &entity.Message{
		MessageType: entity.ResponseMessage,
		Response:    msg,
	}
	msgBytes, err := json.Marshal(clusterMessage)
	if err != nil {
		return err
	}
	b.Producer.Publish(string(topic)+"_resp", msgBytes)

	return nil
}

func (b *ClusterBroker) ConsumeResponse(resp *entity.JsonRpcResponse) {
	b.RespChan <- resp
}

func (b *ClusterBroker) ConsumeRequest(req *entity.JsonRpcRequest) {
	b.ReqChan <- req
}

func (b *ClusterBroker) ConsumeUpdate(update *entity.Update) {
	b.UpdateChan <- update
}

func (b *ClusterBroker) NotifyResponse() <-chan *entity.JsonRpcResponse {
	return b.RespChan
}

func (b *ClusterBroker) NotifyRequest() <-chan *entity.JsonRpcRequest {
	return b.ReqChan
}

func (b *ClusterBroker) NotifyUpdate() <-chan *entity.Update {
	return b.UpdateChan
}
