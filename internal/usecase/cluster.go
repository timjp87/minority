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

func (uc *ClusterUseCase) ForwardConsensusRequest() {
	for {
		req := <-uc.Broker.NotifyRequest()
		log.Info("Forwarding json rpc message", "id", string(req.Body.ID), "version", req.Body.Version, "method", req.Body.Method+"_req")

		resp, err := uc.webapi.Send(req)
		if err != nil {
			log.Error("Failed to send request to execution client", "err", err)
		}

		if resp.Code != 200 {
			log.Error("Execution engine responded with error", "err", resp.Body.Error)
			if err := uc.Broker.PublishResponse(entity.Topic(req.Body.Method)+"_resp", resp); err != nil {
				log.Error("Failed to publish response", "err", err)
			}
		} else {
			log.Info("Publishing execution engine response", "id", string(req.Body.ID), "version", req.Body.Version, "topic", req.Body.Method+"_resp")
			if err := uc.Broker.PublishResponse(entity.Topic(req.Body.Method)+"_resp", resp); err != nil {
				log.Error("Failed to publish response", "err", err)
			}
		}
	}

}

func (uc *ClusterUseCase) MaintainTopology() {
	for {
		update := <-uc.Broker.NotifyUpdate()
		log.Info("Received topology update", "owner", update.Owner, "time", update.Time, "nodes", update.Nodes)
	}

	// Exchange topology messages until a failure or the cluster is torn down
	// var errc chan error
	// for errc == nil {
	// // Iterate over all the known brokers and connect new producers to any
	// // that's not yet connected.
	// var changed bool

	// for name, addr := range c.nodes {
	// // If a producer exists for the specific remote broker, try to ping
	// // it as a health check; removing any failed producers (as well as
	// // the consumer, since we assume they are symmetric).
	// if producer, ok := uc.cluster..prods[name]; ok {
	// if err := producer.Ping(); err != nil {
	// c.logger.Warn("Remote broker became unreachable", "name", name, "err", err)
	// producer.Stop()
	// consumer.DisconnectFromNSQD(addr.String())

	// delete(c.prods, name)
	// c.views[c.broker.Name()][name].Alive = false

	// changed = true // update the node's view of the cluster
	// }
	// }
	// // If a producer does not exist (or was just disconnected), try to
	// // reconnect with it either way.
	// if _, ok := c.prods[name]; !ok {
	// producer, err := c.broker.NewProducer(addr.String())
	// if err != nil {
	// continue
	// }
	// if producer.Ping() != nil {
	// producer.Stop()
	// continue
	// }
	// c.prods[name] = producer

	// self := c.broker.Name()
	// c.views[self][name] = &node{
	// Address: addr.String(),
	// Alive:   true,
	// }
	// changed = true // update the node's view of the cluster
	// }
	// }
	// // If the local view of the network was changed, generate an update message
	// // and publish it with all live producers
	// if changed {
	// msg := &update{
	// Owner: c.broker.Name(),
	// Time:  uint64(time.Now().UnixNano()),
	// Nodes: c.views[c.broker.Name()],
	// }
	// blob, err := json.Marshal(msg)
	// if err != nil {
	// panic(err) // Can't fail, panic during development
	// }
	// for _, producer := range c.prods {
	// if err := producer.Publish(topologyTopic, blob); err != nil {
	// c.logger.Warn("Failed to publish topology update", "err", err)
	// }
	// }
	// // Mark the local view updated to keep the timestamps correlated
	// c.times[msg.Owner] = msg.Time

	// // Finally, report the stats to the user for debugging
	// c.reportStats()
	// }
	// select {
	// case errc = <-c.quit:
	// continue

	// case req := <-c.join:
	// // User request received to join a remote cluster, make sure it's
	// // not yet connected
	// logger := c.logger.New("addr", req.address)
	// logger.Info("Requesting to join remote cluster")

	// var known string
	// for name, addr := range c.nodes {
	// if req.address.String() == addr.String() {
	// known = name
	// break
	// }
	// }
	// if known != "" {
	// logger.Info("Requested peer already known", "name", known)
	// req.result <- fmt.Errorf("peer already known as %s", known)
	// continue
	// }
	// // Peer not yet known, create a temporary producer to check that the
	// // remote peer accepts inbound connections and uses the same secret.
	// producer, err := c.broker.NewProducer(req.address.String())
	// if err != nil {
	// logger.Warn("Failed to connect to remote peer", "err", err)
	// req.result <- err
	// continue
	// }
	// if err := producer.Ping(); err != nil {
	// logger.Warn("Failed to check remote health", "err", err)
	// producer.Stop()
	// req.result <- err
	// continue
	// }
	// // Remote peer alive and mutual authentication passed. Push a topology
	// // update and drop the producer. This will force the members of the
	// // remote cluster to dial back and give us their true external address
	// // and name even if the user supplied something weird.
	// msg := &update{
	// Owner: c.broker.Name(),
	// Time:  c.times[c.broker.Name()],
	// Nodes: c.views[c.broker.Name()],
	// }
	// blob, err := json.Marshal(msg)
	// if err != nil {
	// panic(err) // Can't fail, panic during development
	// }
	// if err := producer.Publish(topologyTopic, blob); err != nil {
	// c.logger.Warn("Failed to publish topology update", "err", err)
	// }
	// producer.Stop()

	// req.result <- nil

	// case update := <-updateCh:
	// // Everyone receives their own updates too. Ignore those as they are
	// // useless (also prevents a duplicate name from overwriting the local
	// // view).
	// logger := c.logger.New("updater", update.Owner, "seqnum", update.Time)
	// if update.Owner == c.broker.Name() {
	// logger.Debug("Ignoring self update")
	// continue
	// }
	// // Topology update received, if it's stale or duplicate, ignore. As
	// // we're publishing and consuming on a full graph, each update will
	// // be received the number of nodes times.
	// if c.times[update.Owner] >= update.Time {
	// logger.Debug("Ignoring stale update")
	// continue
	// }
	// // Topology update newer, integrate it into the local view
	// logger.Info("Updating topology with remote view")

	// c.times[update.Owner] = update.Time
	// c.views[update.Owner] = update.Nodes

	// for name, infos := range update.Nodes {
	// // Make sure the address is valid and drop any invalid entries
	// addr, err := net.ResolveTCPAddr("tcp", infos.Address)
	// if err != nil {
	// logger.Warn("Failed to resolve advertised address", "name", name, "addr", infos.Address, "err", err)
	// delete(c.views[update.Owner], name)
	// continue
	// }
	// // If the remote broker is unknown, start tracking it
	// if _, ok := c.nodes[name]; !ok {
	// // Address clashes might happen when broker names are changed
	// var (
	// renamed  string
	// accepted bool
	// )
	// for oldName, oldAddr := range c.nodes {
	// if oldAddr.String() == infos.Address {
	// // Mark the node renamed - whether we accept it or not -
	// // to avoid discovering it as new too
	// renamed = oldName

	// // If a node advertises the same name as an old one,
	// // check any live connections and reject or accept
	// // based on that.
	// if producer, ok := c.prods[renamed]; ok {
	// if producer.Ping() == nil {
	// // Old connection still valid, ignore advertised address
	// logger.Warn("Rejecting new name for healthy broker", "addr", infos.Address, "old", renamed, "new", name)
	// break
	// }
	// // Ping failed, perhaps the broker just renamed, update all routes
	// logger.Warn("Accepted new name for failing broker", "addr", infos.Address, "old", renamed, "new", name)
	// accepted = true

	// consumer.DisconnectFromNSQD(oldAddr.String())
	// producer.Stop()
	// break
	// }
	// // No previous connection to the broker was maintained,
	// // overwrite the name associated with the address
	// logger.Warn("Accepted new name for dead broker", "addr", infos.Address, "old", renamed, "new", name)
	// accepted = true
	// break
	// }
	// }
	// if renamed != "" && accepted {
	// c.nodes[name] = addr

	// delete(c.nodes, renamed)
	// delete(c.times, renamed)
	// delete(c.views, renamed)
	// delete(c.prods, renamed)

	// delete(c.views[c.broker.Name()], renamed)
	// continue
	// }
	// logger.Info("Discovered new remote broker", "name", name, "addr", infos.Address)
	// c.nodes[name] = addr
	// continue
	// }
	// // If the remote broker was already known, use majority live address
	// if old := c.nodes[name].String(); old != infos.Address {
	// // If we have a live producer, ping the remote node to double-check
	// if producer, ok := c.prods[name]; ok {
	// if producer.Ping() == nil {
	// // Old connection still valid, ignore advertised address
	// logger.Warn("Rejecting new address for healthy broker", "name", name, "old", old, "new", infos.Address)
	// continue
	// }
	// // Ping failed, perhaps the broker just migrated, update all routes
	// logger.Warn("Accepted new address for failing broker", "name", name, "old", old, "new", infos.Address)
	// c.nodes[name] = addr
	// c.views[c.broker.Name()][name].Alive = false

	// consumer.DisconnectFromNSQD(old)
	// producer.Stop()
	// delete(c.prods, name)
	// continue
	// }
	// // No previous connection to the broker was maintained,
	// // overwrite the address associated with the name
	// logger.Warn("Accepted new address for dead broker", "name", name, "old", old, "new", infos.Address)
	// c.nodes[name] = addr
	// }
	// }
	// c.reportStats()
	// }
	// }
	// // If the loop was stopped due to an error, wait for the termination request
	// // and then return. Otherwise, just feed the request a nil error.
	// if errc == nil {
	// errc = <-c.quit
	// }
	// errc <- err
}
