package main

import (
	"os"
	"path/filepath"

	"github.com/ethereum/go-ethereum/log"
	"github.com/karalabe/minority/config"
	"github.com/karalabe/minority/internal/app"
	"github.com/karalabe/minority/pkg/netutil"
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
		Run:   createConfig,
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
	cmdBootnode.Flags().StringVar(&extAddrFlag, "ext.addr", netutil.ExternalAddress(), "Advertised address for remote multiplexers")
	cmdBootnode.Flags().IntVar(&extPortFlag, "ext.port", 0, "Advertised port for remote multiplexers (default = bind.port)")
	cmdBootnode.MarkFlagRequired("node.identity")
	cmdBootnode.MarkFlagRequired("node.secret")

	cmdConsensus := &cobra.Command{
		Use:   "consensus",
		Short: "Run a multiplexer relay for a consensus client",
		Run:   createConfig,
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
	cmdConsensus.Flags().StringVar(&httpAddrFlag, "http.addr", netutil.ExternalAddress(), "HTTP interface for remote multiplexers")
	cmdConsensus.Flags().IntVar(&httpPortFlag, "http.port", 4151, "HTTP interface port for remote multiplexers")
	cmdConsensus.Flags().IntVar(&httpsPortFlag, "https.port", 4152, "HTTPS interface port for remote multiplexers")
	cmdConsensus.Flags().IntVar(&enginePortFlag, "engine.port", 8551, "Interface port for the consensus client to use as engine api")
	cmdConsensus.Flags().StringVar(&engineAddrFlag, "engine.addr", "127.0.0.1", "Interface for the consensus client to use as engine api")
	cmdConsensus.Flags().BoolVar(&enableTLS, "enable.tls", true, "Enable TLS for all http/tcp communication (default = true)")
	cmdConsensus.Flags().BoolVar(&enableHTTP, "enable.http", false, "Enable http interface on NSQ daemon")
	cmdConsensus.Flags().StringVar(&extAddrFlag, "ext.addr", netutil.ExternalAddress(), "Advertised address for remote multiplexers")
	cmdConsensus.Flags().IntVar(&extPortFlag, "ext.port", 0, "Advertised port for remote multiplexers (default = bind.port)")
	cmdConsensus.MarkFlagRequired("node.identity")
	cmdConsensus.MarkFlagRequired("node.secret")

	cmdExecution := &cobra.Command{
		Use:   "execution",
		Short: "Run a multiplexer for relay an execution client",
		Run:   createConfig,
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
	cmdExecution.Flags().StringVar(&httpAddrFlag, "http.addr", netutil.ExternalAddress(), "HTTP interface for remote multiplexers")
	cmdExecution.Flags().IntVar(&httpPortFlag, "http.port", 4151, "HTTP interface port for remote multiplexers")
	cmdExecution.Flags().IntVar(&httpsPortFlag, "https.port", 4152, "HTTP interface port for remote multiplexers")
	cmdExecution.Flags().BoolVar(&enableTLS, "enable.tls", true, "Enable TLS for all http/tcp communication (default = true)")
	cmdExecution.Flags().BoolVar(&enableHTTP, "enable.http", false, "Enable http interface on NSQ daemon (defaut = false)")
	cmdExecution.Flags().IntVar(&enginePortFlag, "engine.port", 8551, "Interface port for the relay to forward requests to an execution client")
	cmdExecution.Flags().StringVar(&engineAddrFlag, "engine.addr", "127.0.0.1", "Interface for the relay to forward requests to an execution client")
	cmdExecution.Flags().StringVar(&extAddrFlag, "ext.addr", netutil.ExternalAddress(), "Advertised address for remote multiplexers")
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
		Run:   createConfig,
	}

	rootCmd := &cobra.Command{Use: "minority"}
	rootCmd.AddCommand(cmdRelay, cmdMerge)
	rootCmd.Execute()
}

func createConfig(cmd *cobra.Command, args []string) {
	cfg := config.NewConfig(cmd, args)
	app.Run(cfg)
}
