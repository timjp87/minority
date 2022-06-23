package app

import (
	"net"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/ethereum/go-ethereum/log"
	"github.com/julienschmidt/httprouter"
	"github.com/karalabe/minority/config"
	controller "github.com/karalabe/minority/internal/controller/http"
	nsq_controller "github.com/karalabe/minority/internal/controller/nsq"
	"github.com/karalabe/minority/internal/entity"
	"github.com/karalabe/minority/internal/usecase"
	"github.com/karalabe/minority/internal/usecase/broker"
	webapi_engine "github.com/karalabe/minority/internal/usecase/webapi"
	"github.com/karalabe/minority/pkg/httpserver"
	"github.com/karalabe/minority/pkg/nsqd"
)

func Run(cfg *config.Config) error {
	// Create new cluster
	self := cfg.Name

	cluster := &entity.Cluster{
		Addr: cfg.ExternalAddr,
		Nodes: map[string]*net.TCPAddr{
			self: cfg.ExternalAddr,
		},
		Times: map[string]uint64{
			self: uint64(time.Now().UnixNano()),
		},
		Views: map[string]map[string]*entity.Node{
			self: make(map[string]*entity.Node),
		},
		Join: make(chan *entity.JoinRequest),
		Quit: make(chan chan error),
		Term: make(chan struct{}),
	}

	// Message Broker
	nsqbroker, err := nsqd.New(cfg.NSQ)
	if err != nil {
		log.Error("Failed to create nsqd", "err", err)
		return err
	}

	clusterBroker, err := broker.New(nsqbroker, cfg.Mode, cluster)
	if err != nil {
		return err
	}

	nsq_controller.AddHandler(clusterBroker, cfg.NSQLookupdHTTP)

	// HTTP Client
	clusterWebAPI := webapi_engine.New(cfg.HTTP)

	// Use case
	clusterUseCase := usecase.New(cfg.App.Mode, cluster, clusterBroker, clusterWebAPI)

	// HTTP Server
	if cfg.Mode == entity.Consensus {
		handler := httprouter.New()
		controller.NewRouter(handler, clusterUseCase)
		log.Info("Starting engine api interface", "endpoint", cfg.HTTP.IP.String()+":"+strconv.Itoa(cfg.HTTP.Port))
		httpServer := httpserver.New(handler, httpserver.Port(strconv.Itoa(cfg.HTTP.Port)))

		// Waiting signal
		interrupt := make(chan os.Signal, 1)
		select {
		case s := <-interrupt:
			log.Info("SIGNAL: " + s.String() + " Stopping minority...")
		case err = <-httpServer.Notify():
			log.Error("Failed to start http server", "err", err)
		}

		// Shutdown
		err = httpServer.Shutdown()
		if err != nil {
			log.Error("app - Run - httpServer.Shutdown:", "err", err)
		}
	}

	// Waiting signal
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt, syscall.SIGTERM)

	s := <-interrupt
	log.Info("SIGNAL: " + s.String() + " Stopping minority...")

	return nil
}
