package config

import (
	"net"
	"strings"

	"github.com/ethereum/go-ethereum/log"
	"github.com/karalabe/minority/internal/entity"
	"github.com/spf13/cobra"
)

type Config struct {
	App
	NSQ
	HTTP
}

// Configuration of the minority client
type App struct {
	Version string
	Mode    entity.RelayMode
}

// Configuration for nsqd
type NSQ struct {
	Name    string
	Secret  string
	Datadir string

	EnableTLS  bool
	EnableHTTP bool

	ExternalAddr   *net.TCPAddr
	TCPListener    *net.TCPAddr
	HTTPListener   *net.TCPAddr
	NSQLookupdTCP  *net.TCPAddr
	NSQLookupdHTTP *net.TCPAddr

	Logger log.Logger
	// Bootnode *net.TCPAddr See if we need bootnode at all with nsqlookup
}

// Address and port for engine API router and client
type HTTP struct {
	*net.TCPAddr
}

func NewConfig(cmd *cobra.Command, args []string) *Config {
	app := App{
		Version: "0.1",
		Mode:    entity.RelayMode(cmd.Use),
	}

	// NSQ
	name, _ := cmd.Flags().GetString("node.identity")
	secret, _ := cmd.Flags().GetString("node.secret")
	datadir, _ := cmd.Flags().GetString("node.datadir")

	extAddr, _ := cmd.Flags().GetString("ext.addr")
	extPort, _ := cmd.Flags().GetInt("ext.port")
	externalAddr := net.TCPAddr{
		IP:   net.ParseIP(extAddr),
		Port: extPort,
	}

	bindAddr, _ := cmd.Flags().GetString("bind.addr")
	bindPort, _ := cmd.Flags().GetInt("bind.port")
	tcpListener := net.TCPAddr{
		IP:   net.ParseIP(bindAddr),
		Port: bindPort,
	}

	httpAddr, _ := cmd.Flags().GetString("http.addr")
	httpPort, _ := cmd.Flags().GetInt("http.port")
	httpListener := net.TCPAddr{
		IP:   net.ParseIP(httpAddr),
		Port: httpPort,
	}

	nsqlookupdAddr, _ := cmd.Flags().GetString("nsqlookupd.addr")
	nsqlookupdTCPPort, _ := cmd.Flags().GetInt("nsqlookupd.tcpport")
	nsqlookupdHTTPPort, _ := cmd.Flags().GetInt("nsqlookupd.httpport")
	nsqlookupdHTTP := net.TCPAddr{
		IP:   net.ParseIP(nsqlookupdAddr),
		Port: nsqlookupdHTTPPort,
	}
	nsqlookupdTCP := net.TCPAddr{
		IP:   net.ParseIP(nsqlookupdAddr),
		Port: nsqlookupdTCPPort,
	}

	enableTLS, _ := cmd.Flags().GetBool("enable.tls")
	enableHTTP, _ := cmd.Flags().GetBool("enable.http")

	nsq := NSQ{
		Name:           name,
		Secret:         secret,
		Datadir:        strings.Replace(datadir, "<uid>", name, -1),
		EnableTLS:      enableTLS,
		EnableHTTP:     enableHTTP,
		TCPListener:    &tcpListener,
		ExternalAddr:   &externalAddr,
		HTTPListener:   &httpListener,
		NSQLookupdTCP:  &nsqlookupdTCP,
		NSQLookupdHTTP: &nsqlookupdHTTP,
		Logger:         log.New(),
	}

	// HTTP
	engineAddr, _ := cmd.Flags().GetString("engine.addr")
	enginePort, _ := cmd.Flags().GetInt("engine.port")

	engineInterface := &net.TCPAddr{
		IP:   net.ParseIP(engineAddr),
		Port: enginePort,
	}

	http := HTTP{
		engineInterface,
	}

	return &Config{
		app,
		nsq,
		http,
	}
}
