// Package broker is a simplification wrapper around the NSQ message broker.
package broker

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httputil"
	"os"
	"path/filepath"

	"github.com/ethereum/go-ethereum/log"
	"github.com/julienschmidt/httprouter"
	"github.com/nsqio/go-nsq"
	"github.com/nsqio/nsq/nsqd"
)

type RelayMode string

const (
	Execution RelayMode = "execution"
	Consensus           = "consensus"
	Bootnode            = "bootnode"
)

// Config is the set of options to fine tune the message broker.
type Config struct {
	Name            string       // Globally unique name for the broker instance
	Mode            RelayMode    // Desired relay mode
	EnableHTTP      bool         // If true the broker allows http requests directly to the NSQ daemon
	EnableTLS       bool         // If true will encrypt traffic between brokers (Default: true)
	Datadir         string       // Data directory to store NSQ related data
	Secret          string       // Shared secret to authenticate into te NSQ cluster
	NSQLoookupdAddr *net.TCPAddr // Entrypoint to nsqlookupd instance
	EngineInterface *net.TCPAddr // Interface over which engine API calls are forwarded
	HTTPListener    *net.TCPAddr // HTTP listener address for NSQ connections
	HTTPSListener   *net.TCPAddr // HTTPS listener address for NSQ connecitons
	TCPListener     *net.TCPAddr // Listener address for NSQ connections

	Logger log.Logger // Logger to allow differentiating brokers if many is embedded
}

// Broker is a locally running message broker that transmits messages between a
// local node (consensus or execution) and remote ones via NSQ.
//
// Internally, each broker runs its own NSQ broker and will maintain a consumer
// status to **all** other known NSQ brokers to exchange node, topic and channel
// topology information.
type Broker struct {
	name string    // Globally unique name for the broker, used by consumer channels
	Mode RelayMode // Which client the broker will relay

	Engine *net.TCPAddr       // Interface over which engine api calls are forwarded
	Router *httprouter.Router // HTTP router which which handels http traffic between engine and consensus client

	tlsCert []byte // Certificate to use for authenticating to other brokers
	tlsKey  []byte // Private key to use for encrypting traffic with other brokers

	daemon *nsqd.NSQD // Message broker embedded in this process
	logger log.Logger // Logger to allow differentiating brokers if many is embedded
}

// New constructs an NSQ broker to communicate with other nodes through.
func New(config *Config) (*Broker, error) {
	// Make sure the config is valid
	if !nsq.IsValidChannelName(config.Name) {
		return nil, fmt.Errorf("invalid broker name '%s', must be alphanumeric", config.Name)
	}
	logger := config.Logger
	if logger == nil {
		logger = log.New()
	}
	logger.Info("Starting message broker", "name", config.Name, "mode", config.Mode, "datadir", config.Datadir, "bind", config.TCPListener, "http", config.HTTPListener)

	// Configure a new NSQ message broker to act as the multiplexer
	opts := nsqd.NewOptions()
	opts.DataPath = config.Datadir

	if config.TCPListener != nil {
		opts.TCPAddress = config.TCPListener.String()
	} else {
		opts.TCPAddress = "0.0.0.0:0" // Default to a random port, public routing
	}

	if config.EnableHTTP {
		if config.EnableTLS {
			opts.HTTPSAddress = config.HTTPSListener.String()
			opts.HTTPAddress = "" // Disable HTTP interface
		} else {
			opts.HTTPAddress = config.HTTPListener.String()

		}
	} else {
		opts.HTTPAddress = ""  // Disable the HTTP interface
		opts.HTTPSAddress = "" // Disable the HTTPS interface
	}

	opts.LogLevel = nsqd.LOG_DEBUG    // We'd like to receive all the broker messages
	opts.Logger = &nsqdLogger{logger} // Replace the default stderr logger with ours

	// Create an ephemeral key on disk and delete as soon as it's loaded. It's
	// needed to work around the NSQ library limitation.
	cert, key := makeTLSCert(config.Secret)
	os.MkdirAll(config.Datadir, 0700)

	if config.EnableTLS {
		if err := ioutil.WriteFile(filepath.Join(config.Datadir, "secret.cert"), cert, 0600); err != nil {
			return nil, err
		}
		defer os.Remove(filepath.Join(config.Datadir, "secret.cert"))

		if err := ioutil.WriteFile(filepath.Join(config.Datadir, "secret.key"), key, 0600); err != nil {
			return nil, err
		}
		defer os.Remove(filepath.Join(config.Datadir, "secret.key"))

		opts.TLSRootCAFile = filepath.Join(config.Datadir, "secret.cert")
		opts.TLSCert = filepath.Join(config.Datadir, "secret.cert")
		opts.TLSKey = filepath.Join(config.Datadir, "secret.key")

		opts.TLSRequired = nsqd.TLSRequired         // Enable TLS encryption for all traffic
		opts.TLSClientAuthPolicy = "require-verify" // Require TLS authentication from all clients
		opts.TLSMinVersion = tls.VersionTLS13       // Require the newest supported TLS protocol
	} else {
		opts.TLSRequired = nsqd.TLSNotRequired
	}

	opts.NSQLookupdTCPAddresses = append(opts.NSQLookupdTCPAddresses, config.NSQLoookupdAddr.String())

	// Create the NSQ daemon and return it without booting up
	daemon, err := nsqd.New(opts)
	if err != nil {
		return nil, err
	}
	go daemon.Main()

	return &Broker{
		name: config.Name,
		Mode: config.Mode,
		Engine: &net.TCPAddr{
			IP:   net.ParseIP(config.EngineInterface.IP.String()),
			Port: config.EngineInterface.Port,
		},
		Router:  httprouter.New(),
		tlsCert: cert,
		tlsKey:  key,
		daemon:  daemon,
		logger:  logger,
	}, nil
}

// StartEngine starts the Engine API handler for consensus clients
func (b *Broker) StartEngine() error {
	log.Info("Starting ethereum json_rpc router", "endpoint", b.Engine.String())

	go func() {
		http.ListenAndServe(b.Engine.String(), b.Router)
	}()

	return nil
}

func (b *Broker) AddEngineHandlerConsensus() error {
	b.logger.Info("Adding handler for calls to the engine api")

	b.Router.POST("/", func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		var rpc jsonrpcMessage
		err := json.NewDecoder(r.Body).Decode(&rpc)

		if err != nil {
			b.logger.Error("Failed to decode json_rpc message")
			w.WriteHeader(400)

			// TODO: Send proper json_rpc error message
			w.Write([]byte("Bad Request"))
		}

		b.logger.Info("Handeling json_rpc request...", "id", rpc.ID, "version", rpc.Version, "method", rpc.Method, "params", rpc.Params, "host", r.RemoteAddr)

		producer, err := b.NewProducer(b.daemon.RealTCPAddr().String())
		if err != nil {
			b.logger.Error("Failed to create producer", "err", err)
		}

		pubMsg, _ := httputil.DumpRequest(r, true)
		producer.Publish(rpc.Method, pubMsg)

		resp := make(chan jsonrpcMessage, 1)
		go b.consumeFromTopic(rpc.Method, resp)

		msg := <-resp
		respBytes, err := json.Marshal(msg)
		if err != nil {
			b.logger.Error("Failed to marshal message to JSON", "err", err)
		}
		b.logger.Info("Responding with", "msg", msg)
		w.Write(respBytes)
	})

	return nil
}

func (b *Broker) consumeFromTopic(t string, respChan chan jsonrpcMessage) {
	c, err := b.NewConsumer(t)
	if err != nil {
		b.logger.Error("Failed to create consumer", "err", err)
	}

	c.AddHandler(nsq.HandlerFunc(func(message *nsq.Message) error {
		var rpcMsg jsonrpcMessage
		err := json.Unmarshal(message.Body, &rpcMsg)
		if err != nil {
			b.logger.Error("Error unmarshling message to json_rpc message", "err", err)
		}

		respChan <- rpcMsg

		return nil
	}))
}

// Close terminates the NSQ daemon.
func (b *Broker) Close() error {
	b.daemon.Exit()
	return nil
}

// Name returns the globally unique (user assigned) name of the broker.
func (b *Broker) Name() string {
	return b.name
}

// Port returns the local port number the broker is listening on.
func (b *Broker) Port() int {
	return b.daemon.RealTCPAddr().Port
}

// NewProducer creates a new producer connected to the specified remote (or local)
// NSQD daemon instance.
func (b *Broker) NewProducer(addr string) (*nsq.Producer, error) {
	config := nsq.NewConfig()
	config.Snappy = true
	config.TlsV1 = true
	config.TlsConfig = makeTLSConfig(b.tlsCert, b.tlsKey)

	producer, err := nsq.NewProducer(addr, config)
	if err != nil {
		return nil, err
	}
	producer.SetLogger(&nsqProducerLogger{b.logger}, nsq.LogLevelDebug)

	return producer, nil
}

// NewConsumer creates a new consumer configured to authenticate into the broker
// cluster and to listen for specific events; though the connectivity itself is
// left for the outside caller.
func (b *Broker) NewConsumer(topic string) (*nsq.Consumer, error) {
	config := nsq.NewConfig()
	config.Snappy = true
	config.TlsV1 = true
	config.TlsConfig = makeTLSConfig(b.tlsCert, b.tlsKey)

	consumer, err := nsq.NewConsumer(topic, b.name, config)
	if err != nil {
		return nil, err
	}
	consumer.SetLogger(&nsqConsumerLogger{b.logger}, nsq.LogLevelDebug)

	return consumer, nil
}
