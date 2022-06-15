package main

import (
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"strings"

	"github.com/ethereum/go-ethereum/log"
	"github.com/karalabe/minority/broker"
	"github.com/karalabe/minority/cluster"
	"github.com/spf13/cobra"
)

var (
	identityFlag        string
	datadirFlag         string
	secretFlag          string
	bootnodeFlag        string
	lookupdAddrFlag     string
	lookupdHttpPortFlag int
	lookupdTcpPortFlag  int
	bindAddrFlag        string
	bindPortFlag        int
	httpAddrFlag        string
	httpPortFlag        int
	httpsPortFlag       int
	engineAddrFlag      string
	enginePortFlag      int
	enableTLS           bool
	enableHTTP          bool
	extAddrFlag         string
	extPortFlag         int
)

func main() {
	// Configure the logger to print everything
	log.Root().SetHandler(log.LvlFilterHandler(log.LvlInfo, log.StreamHandler(os.Stderr, log.TerminalFormat(true))))

	// Create the commands to run relay instances
	cmdBootnode := &cobra.Command{
		Use:   "bootnode",
		Short: "Run a multiplexer relay to upkeep the cluster",
		Run:   runRelay,
	}
	cmdBootnode.Flags().StringVar(&identityFlag, "node.identity", "", "Unique identifier for this node across the entire cluster")
	cmdBootnode.Flags().StringVar(&datadirFlag, "node.datadir", filepath.Join(os.Getenv("HOME"), ".minority", "<uid>"), "Folder to persist state through restarts")
	cmdBootnode.Flags().StringVar(&secretFlag, "node.secret", "", "Shared secret to authenticate and encrypt with")
	cmdBootnode.Flags().StringVar(&bootnodeFlag, "node.boot", "", "Entrypoint into an existing multiplexer cluster")
	cmdBootnode.Flags().StringVar(&lookupdAddrFlag, "nsqlookupd.addr", "127.0.0.1", "Iterface address to connect to nsqlookupd")
	cmdBootnode.Flags().IntVar(&lookupdTcpPortFlag, "nsqlookupd.tcpport", 4160, "Iterface address to connect to nsqlookupd")
	cmdBootnode.Flags().IntVar(&lookupdHttpPortFlag, "nsqlookupd.httpport", 4161, "Iterface address to connect to nsqlookupd")
	cmdBootnode.Flags().StringVar(&bindAddrFlag, "bind.addr", "0.0.0.0", "Listener interface for remote multiplexers")
	cmdBootnode.Flags().IntVar(&bindPortFlag, "bind.port", 4150, "Listener port for remote multiplexers")
	cmdBootnode.Flags().StringVar(&extAddrFlag, "ext.addr", externalAddress(), "Advertised address for remote multiplexers")
	cmdBootnode.Flags().IntVar(&extPortFlag, "ext.port", 0, "Advertised port for remote multiplexers (default = bind.port)")
	cmdBootnode.MarkFlagRequired("node.identity")
	cmdBootnode.MarkFlagRequired("node.secret")

	cmdConsensus := &cobra.Command{
		Use:   "consensus",
		Short: "Run a multiplexer relay for a consensus client",
		Run:   runRelay,
	}
	cmdConsensus.Flags().StringVar(&identityFlag, "node.identity", "", "Unique identifier for this node across the entire cluster")
	cmdConsensus.Flags().StringVar(&datadirFlag, "node.datadir", filepath.Join(os.Getenv("HOME"), ".minority", "<uid>"), "Folder to persist state through restarts")
	cmdConsensus.Flags().StringVar(&secretFlag, "node.secret", "", "Shared secret to authenticate and encrypt with")
	cmdConsensus.Flags().StringVar(&bootnodeFlag, "node.boot", "", "Entrypoint into an existing multiplexer cluster")
	cmdConsensus.Flags().StringVar(&lookupdAddrFlag, "nsqlookupd.addr", "127.0.0.1", "Iterface address to connect to nsqlookupd")
	cmdConsensus.Flags().IntVar(&lookupdTcpPortFlag, "nsqlookupd.tcpport", 4160, "Iterface address to connect to nsqlookupd")
	cmdConsensus.Flags().IntVar(&lookupdHttpPortFlag, "nsqlookupd.httpport", 4161, "Iterface address to connect to nsqlookupd")
	cmdConsensus.Flags().StringVar(&bindAddrFlag, "bind.addr", "0.0.0.0", "Listener interface for remote multiplexers")
	cmdConsensus.Flags().IntVar(&bindPortFlag, "bind.port", 4150, "Listener port for remote multiplexers")
	cmdConsensus.Flags().StringVar(&httpAddrFlag, "http.addr", externalAddress(), "HTTP interface for remote multiplexers")
	cmdConsensus.Flags().IntVar(&httpPortFlag, "http.port", 4151, "HTTP interface port for remote multiplexers")
	cmdConsensus.Flags().IntVar(&httpsPortFlag, "https.port", 4152, "HTTPS interface port for remote multiplexers")
	cmdConsensus.Flags().IntVar(&enginePortFlag, "engine.port", 8551, "Interface port for the consensus client to use as engine api")
	cmdConsensus.Flags().StringVar(&engineAddrFlag, "engine.addr", "127.0.0.1", "Interface for the consensus client to use as engine api")
	cmdConsensus.Flags().BoolVar(&enableTLS, "enable.tls", true, "Enable TLS for all http/tcp communication (default = true)")
	cmdConsensus.Flags().BoolVar(&enableHTTP, "enable.http", false, "Enable http interface on NSQ daemon")
	cmdConsensus.Flags().StringVar(&extAddrFlag, "ext.addr", externalAddress(), "Advertised address for remote multiplexers")
	cmdConsensus.Flags().IntVar(&extPortFlag, "ext.port", 0, "Advertised port for remote multiplexers (default = bind.port)")
	cmdConsensus.MarkFlagRequired("node.identity")
	cmdConsensus.MarkFlagRequired("node.secret")

	cmdExecution := &cobra.Command{
		Use:   "execution",
		Short: "Run a multiplexer for relay an execution client",
		Run:   runRelay,
	}
	cmdExecution.Flags().StringVar(&identityFlag, "node.identity", "", "Unique identifier for this node across the entire cluster")
	cmdExecution.Flags().StringVar(&datadirFlag, "node.datadir", filepath.Join(os.Getenv("HOME"), ".minority", "<uid>"), "Folder to persist state through restarts")
	cmdExecution.Flags().StringVar(&secretFlag, "node.secret", "", "Shared secret to authenticate and encrypt with")
	cmdExecution.Flags().StringVar(&bootnodeFlag, "node.boot", "", "Entrypoint into an existing multiplexer cluster")
	cmdExecution.Flags().StringVar(&lookupdAddrFlag, "nsqlookupd.addr", "127.0.0.1", "Iterface address to connect to nsqlookupd")
	cmdExecution.Flags().IntVar(&lookupdTcpPortFlag, "nsqlookupd.tcpport", 4160, "Iterface address to connect to nsqlookupd")
	cmdExecution.Flags().IntVar(&lookupdHttpPortFlag, "nsqlookupd.httpport", 4161, "Iterface address to connect to nsqlookupd")
	cmdExecution.Flags().StringVar(&bindAddrFlag, "bind.addr", "0.0.0.0", "Listener interface for remote multiplexers")
	cmdExecution.Flags().IntVar(&bindPortFlag, "bind.port", 4150, "Listener port for remote multiplexers")
	cmdExecution.Flags().StringVar(&httpAddrFlag, "http.addr", externalAddress(), "HTTP interface for remote multiplexers")
	cmdExecution.Flags().IntVar(&httpPortFlag, "http.port", 4151, "HTTP interface port for remote multiplexers")
	cmdExecution.Flags().IntVar(&httpsPortFlag, "https.port", 4152, "HTTP interface port for remote multiplexers")
	cmdExecution.Flags().BoolVar(&enableTLS, "enable.tls", true, "Enable TLS for all http/tcp communication (default = true)")
	cmdExecution.Flags().BoolVar(&enableHTTP, "enable.http", false, "Enable http interface on NSQ daemon (defaut = false)")
	cmdExecution.Flags().IntVar(&enginePortFlag, "engine.port", 8551, "Interface port for the relay to forward requests to an execution client")
	cmdExecution.Flags().StringVar(&engineAddrFlag, "engine.addr", "127.0.0.1", "Interface for the relay to forward requests to an execution client")
	cmdExecution.Flags().StringVar(&extAddrFlag, "ext.addr", externalAddress(), "Advertised address for remote multiplexers")
	cmdExecution.Flags().IntVar(&extPortFlag, "ext.port", 0, "Advertised port for remote multiplexers (default = bind.port)")
	cmdExecution.MarkFlagRequired("node.identity")
	cmdExecution.MarkFlagRequired("node.secret")

	cmdRelay := &cobra.Command{
		Use:   "relay",
		Short: "Start a multiplexer relaying Ethereum APIs",
	}
	cmdRelay.AddCommand(cmdBootnode, cmdConsensus, cmdExecution)

	// Create the commands to merge relay instances
	cmdMerge := &cobra.Command{
		Use:   "merge",
		Short: "Request merging two relay clusters",
		Run:   runMerge,
	}

	rootCmd := &cobra.Command{Use: "minority"}
	rootCmd.AddCommand(cmdRelay, cmdMerge)
	rootCmd.Execute()
}

func runRelay(cmd *cobra.Command, args []string) {
	// Configure and start the message broker

	brokerConfig := &broker.Config{
		Name:       identityFlag,
		Mode:       broker.RelayMode(cmd.Use),
		Datadir:    strings.Replace(datadirFlag, "<uid>", identityFlag, -1),
		Secret:     secretFlag,
		EnableTLS:  enableTLS,
		EnableHTTP: enableHTTP,
		NSQLoookupdTCPAddr: &net.TCPAddr{
			IP:   net.ParseIP(lookupdAddrFlag),
			Port: lookupdTcpPortFlag,
		},
		NSQLoookupdHTTPAddr: &net.TCPAddr{
			IP:   net.ParseIP(lookupdAddrFlag),
			Port: lookupdHttpPortFlag,
		},
		EngineInterface: &net.TCPAddr{
			IP:   net.ParseIP(engineAddrFlag),
			Port: enginePortFlag,
		},
		HTTPListener: &net.TCPAddr{
			IP:   net.ParseIP(httpAddrFlag),
			Port: httpPortFlag,
		},
		HTTPSListener: &net.TCPAddr{
			IP:   net.ParseIP(httpAddrFlag),
			Port: httpsPortFlag,
		},
		TCPListener: &net.TCPAddr{
			IP:   net.ParseIP(bindAddrFlag),
			Port: bindPortFlag,
		},
	}

	broker, err := broker.New(brokerConfig)
	if err != nil {
		log.Crit("Failed to start message broker", "err", err)
	}
	defer broker.Close()

	// Configure and start the cluster manager
	if extPortFlag == 0 {
		extPortFlag = bindPortFlag
	}

	clusterConfig := &cluster.Config{
		External: &net.TCPAddr{
			IP:   net.ParseIP(extAddrFlag),
			Port: extPortFlag,
		},
	}

	cluster, err := cluster.New(clusterConfig, broker)
	if err != nil {
		log.Crit("Failed to start message cluster", "err", err)
	}
	defer cluster.Close()

	// If a bootnode was specified, explicitly connect to it
	if bootnodeFlag != "" {
		bootnodeAddr, err := net.ResolveTCPAddr("tcp", bootnodeFlag)
		if err != nil {
			log.Crit("Failed to resolve bootnode address", "err", err)
		}
		if err := cluster.Join(bootnodeAddr); err != nil {
			log.Crit("Failed to join to bootnode", "err", err)
		}
	}

	// Wait until the process is terminated
	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, os.Interrupt)
	<-signalCh
}

func runMerge(cmd *cobra.Command, args []string) {
	// Configure and start the message broker
	brokerConfig := &broker.Config{
		Name:    identityFlag,
		Datadir: strings.Replace(datadirFlag, "<uid>", identityFlag, -1),
		Secret:  secretFlag,
		TCPListener: &net.TCPAddr{
			IP:   net.ParseIP(bindAddrFlag),
			Port: bindPortFlag,
		},
	}
	broker, err := broker.New(brokerConfig)
	if err != nil {
		log.Crit("Failed to start message broker", "err", err)
	}
	defer broker.Close()

	panic("todo")
}

// externalAddress iterates over all the network interfaces of the machine and
// returns the first non-loopback one (or the loopback if none can be found).
func externalAddress() string {
	ifaces, err := net.Interfaces()
	if err != nil {
		log.Crit("Failed to retrieve network interfaces", "err", err)
	}
	for _, iface := range ifaces {
		// Skip disconnected and loopback interfaces
		if iface.Flags&net.FlagUp == 0 {
			continue
		}
		if iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			log.Warn("Failed to retrieve network addresses", "err", err)
			continue
		}
		for _, addr := range addrs {
			switch v := addr.(type) {
			case *net.IPNet:
				return v.IP.String()
			case *net.IPAddr:
				return v.IP.String()
			}
		}
	}
	return "127.0.0.1"
}
