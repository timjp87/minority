package entity

// Update is a message sent to all other relays in the cluster to share
// the local views and allow everyone to construct an accurate global view.
type Update struct {
	Owner string           `json:"owner"` // Unique ID of the node that broadcast this update
	Time  uint64           `json:"time"`  // Timestamp in ns of this update (used as seq number)
	Nodes map[string]*Node `json:"nodes"` // Node ID to NSQD address mapping
}

// Node represents a node in the cluster.
type Node struct {
	Address string `json:"address"` // NSQD address to reach the broker through
	Alive   bool   `json:"alive"`   // Whether the address is reachable or not
}
