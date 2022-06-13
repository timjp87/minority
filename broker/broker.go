// Package broker is a simplification wrapper around the NSQ message broker.
package broker

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"

	"github.com/ethereum/go-ethereum/log"
	"github.com/julienschmidt/httprouter"
	"github.com/karalabe/minority/common"
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
	Name                string       // Globally unique name for the broker instance
	Mode                RelayMode    // Desired relay mode
	EnableHTTP          bool         // If true the broker allows http requests directly to the NSQ daemon
	EnableTLS           bool         // If true will encrypt traffic between brokers (Default: true)
	Datadir             string       // Data directory to store NSQ related data
	Secret              string       // Shared secret to authenticate into te NSQ cluster
	NSQLoookupdTCPAddr  *net.TCPAddr // TCP Entrypoint to nsqlookupd instance
	NSQLoookupdHTTPAddr *net.TCPAddr // TCP Entrypoint to nsqlookupd instance
	EngineInterface     *net.TCPAddr // Interface over which engine API calls are forwarded
	HTTPListener        *net.TCPAddr // HTTP listener address for NSQ connections
	HTTPSListener       *net.TCPAddr // HTTPS listener address for NSQ connecitons
	TCPListener         *net.TCPAddr // Listener address for NSQ connections

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

	Engine             *net.TCPAddr             // Interface over which engine api calls are forwarded
	Router             *httprouter.Router       // HTTP router which which handels http traffic between engine and consensus client
	Consumers          map[string]*nsq.Consumer // Consumers for each json_rpc topic
	NSQLookupdHTTPAddr *net.TCPAddr             // Address of nsqlookup do to reqister consumers with to discover producers

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

	opts.NSQLookupdTCPAddresses = append(opts.NSQLookupdTCPAddresses, config.NSQLoookupdTCPAddr.String())

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
		Consumers: make(map[string]*nsq.Consumer),
		Router:    httprouter.New(),
		tlsCert:   cert,
		tlsKey:    key,
		daemon:    daemon,
		logger:    logger,
	}, nil
}

// StartEngine starts the engine API interface for consensus clients
func (b *Broker) StartEngine() error {
	log.Info("Starting ethereum json_rpc router", "endpoint", b.Engine.String())

	go func() {
		err := http.ListenAndServe(b.Engine.String(), b.Router)
		if err != nil {
			b.logger.Crit("Could not start engine api interface", "err", err)
			panic(err)
		}
	}()

	return nil
}

func (b *Broker) AddExectionRequestResponseHandler() error {

	// Create a consumer for each json_rpc req topic to receive requests from the consensus clients
	for _, topic := range common.EthereumTopics {
		consumer, err := b.NewConsumer(topic + "_req")
		if err != nil {
			b.logger.Error("Failed to create consumer", "topic", topic+"_req")
			return err
		}

		consumer.AddHandler(nsq.HandlerFunc(func(message *nsq.Message) error {
			// We assumed that the consensus relay published a raw http request from the consensus client.
			// After parsing it we can refer to its body to decode the json_rpc request.
			consensusRequest, err := http.ReadRequest(bufio.NewReader(bytes.NewReader(message.Body)))
			if err != nil {
				b.logger.Error("Failed to read NSQ message as consensus client http request", "err", err)
				return err
			}

			// Try to decode the body of the request to the json_rpc request from consensus client
			// so we can refer to its attributes.
			var rpcMsg jsonrpcMessage
			if err := json.NewDecoder(consensusRequest.Body).Decode(&rpcMsg); err != nil {
				b.logger.Error("Failed to decode to json_rpc message", "err", err)
				return err
			}

			b.logger.Info("Handeling nsq message", "topic", rpcMsg.Method)

			// Start constructing the request to pass to the engine endpoint defined for the execution engine relay
			// We can not simply pass the raw http request we got to http.DefaultClient.Do() as we need to set
			// the new destionation addres.
			request := http.Request{}
			request.URL, err = url.Parse("http://" + b.Engine.String())
			if err != nil {
				b.logger.Error("Failed to parse engine api address to forward request", "addr", "http://"+b.Engine.String())
				return err
			}
			request.Header = consensusRequest.Header
			request.Body = consensusRequest.Body
			resp, err := http.DefaultClient.Do(&request)
			if err != nil {
				b.logger.Error("Failed to make HTTP request to execution engine", "err", err)
				return err
			}

			//Publish back the raw http reponse we received from the execution client to the response
			// topic for this json_rpc method.
			r, _ := httputil.DumpResponse(resp, true)
			producer, err := b.NewProducer(b.daemon.RealTCPAddr().String())
			if err != nil {
				b.logger.Error("Failed to create producer", "err", err)
				return err
			}
			err = producer.Publish(rpcMsg.Method+"_resp", r)
			if err != nil {
				b.logger.Error("Failed to publish execution engine response", "topic", rpcMsg.Method+"_resp")
				return err
			}
			return nil
		}))

		//Look for nsqd nodes pubilshing to our requet topic via nsqlookup. These might not exist yet. Think about
		// how to solve the chicken & egg problem. Depending on which relay was started and processed first. Otherwise
		// message propagation is slowed because we need to wait for topic and nodes are discovered while the token expires.
		err = consumer.ConnectToNSQLookupd(b.NSQLookupdHTTPAddr.String())
		if err != nil {
			b.logger.Error("Failed to connected to broker", "broker", b.daemon.RealTCPAddr().String(), "err", err)
		}

		// Store the consumer in our topic to consumer map at the broker
		b.Consumers[topic] = consumer
	}

	return nil
}

func (b *Broker) AddConsensusRequestResponseHandler() error {
	b.logger.Info("Adding handler for calls to the engine api")

	b.Router.POST("/", func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		// We dump the http request with body here so we can publish it without having the body
		// be consumed be the json decoder.
		dump, _ := httputil.DumpRequest(r, true)

		// Decode the body to a json_rpc message so we can refer to its attributes
		var rpc jsonrpcMessage
		err := json.NewDecoder(r.Body).Decode(&rpc)
		if err != nil {
			b.logger.Error("Failed to decode json_rpc message")
			w.WriteHeader(400)

			// TODO: Send proper json_rpc error message
			w.Write([]byte("Bad Request. Not a JSON_RPC format message."))
		}

		b.logger.Info("Handeling json_rpc request...", "id", string(rpc.ID), "version", rpc.Version, "method", rpc.Method, "params", string(rpc.Params), "host", r.RemoteAddr)

		// Create producer at the embedded NSQ daemon and publish the the request topic for this rpc method
		producer, err := b.NewProducer(b.daemon.RealTCPAddr().String())
		if err != nil {
			b.logger.Error("Failed to create producer", "err", err)
		}
		producer.Publish(rpc.Method+"_req", dump)

		// We wait until a message arrives at the corresponding response topic. This topic might not exist
		// at the NSQ lookup daemon, yet. Execution relay creates it at it's NSQ daemon when publishing the
		// response from the actual execution client. Probably source of delay when topic information isn't
		// in sync and needs to be discovered eventually. Probably causes the JWT token to expire in the meantime.
		respChan := make(chan *nsq.Message, 1)
		go b.ConsumeNSQMessageFromTopic(rpc.Method+"_resp", respChan)
		msg := <-respChan

		// Execution client sent back raw http response from execution client. We parse it so we can set our header
		// status code and content-type and body in the response back to the consensus client.
		response, err := http.ReadResponse(bufio.NewReader(bytes.NewReader(msg.Body)), r)
		if err != nil {
			b.logger.Error("Failed to parse response into http.Reponse", "topic", rpc.Method, "err", err)
		}

		w.Header().Add("Authorization", response.Header.Get("Authorization"))
		w.Header().Add("Content-Type", response.Header.Get("Content-Type"))
		w.WriteHeader(response.StatusCode)
		respBody, err := ioutil.ReadAll(response.Body)
		if err != nil {
			b.logger.Error("Failed to read body of http response received", "err", err)
		}
		b.logger.Info("Responding to json_rpc request...", "id", string(rpc.ID), "version", rpc.Version, "method", rpc.Method, "params", string(rpc.Params), "host", r.RemoteAddr)
		w.Write(respBody)
	})

	return nil
}

//Subscribes to a certain topic via nsqlookupd and sends the message back via the supplied channel
func (b *Broker) ConsumeNSQMessageFromTopic(t string, respChan chan *nsq.Message) error {
	c, err := b.NewConsumer(t)
	if err != nil {
		b.logger.Error("Failed to create consumer", "err", err)
	}

	c.AddHandler(nsq.HandlerFunc(func(message *nsq.Message) error {
		respChan <- message
		return nil
	}))

	err = c.ConnectToNSQLookupd(b.NSQLookupdHTTPAddr.String())
	if err != nil {
		b.logger.Error("Failed to connect consumer to nsqd", "err", err)
		return err
	}

	return nil
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
