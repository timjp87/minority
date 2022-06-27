package nsq_controller

import (
	"bytes"
	"encoding/json"
	"net"

	"github.com/ethereum/go-ethereum/log"
	"github.com/karalabe/minority/internal/entity"
	"github.com/karalabe/minority/internal/usecase/broker"
	"github.com/nsqio/go-nsq"
)

func AddHandler(c *broker.ClusterBroker, nsqlookupdhttp *net.TCPAddr) {
	for key, consumer := range c.Consumers {
		log.Info("Setting up handler", "topic", key)
		consumer.AddHandler(nsq.HandlerFunc(func(message *nsq.Message) error {
			var clusterMsg *entity.Message

			decoder := json.NewDecoder(bytes.NewReader(message.Body))
			decoder.DisallowUnknownFields()

			if err := decoder.Decode(&clusterMsg); err != nil {
				log.Error("Failed to decode nsq message to cluster message", "err", err)
			}

			switch clusterMsg.MessageType {
			case "request":
				if clusterMsg.Request.Body.Method != "announce" {
					c.ConsumeRequest(clusterMsg.Request)
				}
			case "response":
				if clusterMsg.Response.Body.Method != "announce" {
					c.ConsumeResponse(clusterMsg.Response)
				}
			case "update":
				if clusterMsg.Update.Owner != c.Name {
					c.ConsumeUpdate(clusterMsg.Update)
				}
			}

			return nil
		}))
		consumer.ConnectToNSQLookupd(nsqlookupdhttp.String())
	}
}
