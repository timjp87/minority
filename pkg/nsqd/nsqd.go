package nsqd

import (
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/karalabe/minority/config"
	"github.com/karalabe/minority/pkg/crypto"
	nsqlogger "github.com/karalabe/minority/pkg/logger"
	"github.com/nsqio/go-nsq"
	"github.com/nsqio/nsq/nsqd"
)

type NSQD struct {
	Name   string
	Daemon *nsqd.NSQD
}

func New(cfg config.NSQ) (*NSQD, error) {
	// Make sure the config is valid
	if !nsq.IsValidChannelName(cfg.Name) {
		return nil, fmt.Errorf("invalid broker name '%s', must be alphanumeric", cfg.Name)
	}

	logger := cfg.Logger
	logger.Info("Starting message broker", "name", cfg.Name, "datadir", cfg.Datadir, "bind", cfg.TCPListener, "http", cfg.HTTPListener)

	// Configure a new NSQ message broker to act as the multiplexer
	opts := nsqd.NewOptions()
	opts.DataPath = cfg.Datadir

	if cfg.TCPListener != nil {
		opts.TCPAddress = cfg.TCPListener.String()
	} else {
		opts.TCPAddress = "0.0.0.0:0" // Default to a random port, public routing
	}

	if cfg.EnableHTTP {
		if cfg.EnableTLS {
			opts.HTTPSAddress = cfg.HTTPListener.String()
			opts.HTTPAddress = "" // Disable HTTP interface
		} else {
			opts.HTTPAddress = cfg.HTTPListener.String()

		}
	} else {
		opts.HTTPAddress = ""  // Disable the HTTP interface
		opts.HTTPSAddress = "" // Disable the HTTPS interface
	}

	opts.LogLevel = nsqd.LOG_INFO                       // We'd like to receive all the broker messages
	opts.Logger = &nsqlogger.NSQDLogger{Logger: logger} // Replace the default stderr logger with ours

	// Create an ephemeral key on disk and delete as soon as it's loaded. It's
	// needed to work around the NSQ library limitation.
	cert, key := crypto.MakeTLSCert(cfg.Secret)
	os.MkdirAll(cfg.Datadir, 0700)

	if cfg.EnableTLS {
		if err := ioutil.WriteFile(filepath.Join(cfg.Datadir, "secret.cert"), cert, 0600); err != nil {
			return nil, err
		}
		defer os.Remove(filepath.Join(cfg.Datadir, "secret.cert"))

		if err := ioutil.WriteFile(filepath.Join(cfg.Datadir, "secret.key"), key, 0600); err != nil {
			return nil, err
		}
		defer os.Remove(filepath.Join(cfg.Datadir, "secret.key"))

		opts.TLSRootCAFile = filepath.Join(cfg.Datadir, "secret.cert")
		opts.TLSCert = filepath.Join(cfg.Datadir, "secret.cert")
		opts.TLSKey = filepath.Join(cfg.Datadir, "secret.key")

		opts.TLSRequired = nsqd.TLSRequired         // Enable TLS encryption for all traffic
		opts.TLSClientAuthPolicy = "require-verify" // Require TLS authentication from all clients
		opts.TLSMinVersion = tls.VersionTLS13       // Require the newest supported TLS protocol
	} else {
		opts.TLSRequired = nsqd.TLSNotRequired
	}

	opts.NSQLookupdTCPAddresses = append(opts.NSQLookupdTCPAddresses, cfg.NSQLookupdTCP.String())

	// Create the NSQ daemon and return it without booting up
	daemon, err := nsqd.New(opts)
	if err != nil {
		return nil, err
	}
	go daemon.Main()

	nsqConfig := nsq.NewConfig()
	nsqConfig.Snappy = true
	nsqConfig.TlsV1 = true
	nsqConfig.TlsConfig = crypto.MakeTLSConfig(cert, key)

	producer, err := nsq.NewProducer(cfg.TCPListener.String(), nsqConfig)
	if err != nil {
		return nil, err
	}
	producer.SetLogger(&nsqlogger.NSQProducerLogger{Logger: logger}, nsq.LogLevelInfo)

	return &NSQD{
		Name:   cfg.Name,
		Daemon: daemon,
	}, nil
}
