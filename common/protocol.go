package common

// topologyTopic is the NSQ topic where topology broadcasts are published.
const TopologyTopic = ".topology#ephemeral"

var EthereumTopics [13]string = [13]string{
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

// update is a message sent to all other relays in the cluster to share
// the local views and allow everyone to construct an accurate global view.
type Update struct {
	Owner string           `json:"owner"` // Unique ID of the node that broadcast this update
	Time  uint64           `json:"time"`  // Timestamp in ns of this update (used as seq number)
	Nodes map[string]*Node `json:"nodes"` // Node ID to NSQD address mapping
}

// node
type Node struct {
	Address string `json:"address"` // NSQD address to reach the broker through
	Alive   bool   `json:"alive"`   // Whether the address is reachable or not
}
