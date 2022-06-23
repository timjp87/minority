package nsq_controller

import (
	"encoding/json"
	"net"
	"strings"

	"github.com/ethereum/go-ethereum/log"
	"github.com/karalabe/minority/internal/entity"
	"github.com/karalabe/minority/internal/usecase/broker"
	"github.com/nsqio/go-nsq"
)

func AddHandler(c *broker.ClusterBroker, nsqlookupdhttp *net.TCPAddr) {
	for key, consumer := range c.Consumers {
		log.Info("Setting up handler", "topic", key)
		consumer.SetLoggerLevel(nsq.LogLevelInfo)
		consumer.AddHandler(nsq.HandlerFunc(func(message *nsq.Message) error {
			switch strings.Contains(string(key), "_req") {
			case true:
				var rpcReq entity.JsonRpcRequest
				if err := json.Unmarshal(message.Body, &rpcReq); err != nil {
					return err
				}

				// Discards messages that were published in precreation of topic
				if rpcReq.Body.Method != "announce" {
					log.Info("Handeling nsq message...", "method", rpcReq.Body.Method)
					c.ConsumeRequest(&rpcReq)
				}

			case false:
				// When it's not a request topic it could be about toplogy if not it's response topic
				if strings.Contains(string(key), "topology") {
					var toplogyUpdate entity.Update
					if err := json.Unmarshal(message.Body, &toplogyUpdate); err != nil {
						return err
					}
					log.Info("Handeling topology update", "owner", toplogyUpdate.Owner, "time", toplogyUpdate.Time)
					c.ConsumeUpdate(&toplogyUpdate)
				} else {
					var rpcResp entity.JsonRpcResponse
					if err := json.Unmarshal(message.Body, &rpcResp); err != nil {
						return err
					}

					// Discards messages published in topic precreation
					if rpcResp.Body.Method != "announce" {
						log.Info("Handeling nsq message...", "method", rpcResp.Body.Method)
						c.ConsumeResponse(&rpcResp)

					}
				}

			}
			return nil
		}))
		consumer.ConnectToNSQLookupd(nsqlookupdhttp.String())
	}
}
