package entity

type Topic string

// TopologyTopic is the topic where topology broadcasts are published.
const TopologyTopic Topic = ".topology#ephemeral"

// EthereumTopcis are the topic where rpc messages are published/consumed.
var EthereumTopics [13]Topic = [13]Topic{
	"eth_blockNumber",
	"eth_call",
	"eth_chainId",
	"eth_getCode",
	"eth_getBlockByHash",
	"eth_getBlockByNumber",
	"eth_getLogs",
	"eth_sendRawTransaction",
	"eth_syncing",
	"engine_newPayloadV1",
	"engine_forkchoiceUpdatedV1",
	"engine_getPayloadV1",
	"engine_exchangeTransitionConfigurationV1",
}
