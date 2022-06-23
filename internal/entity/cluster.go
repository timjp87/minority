package entity

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"log"
	"net"
	"sort"
	"strconv"
	"time"

	"github.com/olekukonko/tablewriter"
)

type RelayMode string

const (
	Execution RelayMode = "execution"
	Consensus RelayMode = "consensus"
	Bootnode  RelayMode = "bootnode"
)

// API request to have the local cluster join with a remote one.
type JoinRequest struct {
	address *net.TCPAddr // Remote address to join with
	result  chan error   // Result of the join operation
}

type Cluster struct {
	Addr *net.TCPAddr // External address of the local broker

	Nodes map[string]*net.TCPAddr     // Known remote relays and their addresses
	Times map[string]uint64           // Timestamps of the last broker updates
	Views map[string]map[string]*Node // Remote views of the broker cluster

	Join chan *JoinRequest // Channel for requesting joining a remote cluster

	Quit chan chan error // Termination channel to tear down the node
	Term chan struct{}   // Notification channel of termination

	Logger log.Logger
}

// reportStats is a debug method to print the current cluster topology and any
// other stats that might be useful.
func (c *Cluster) ReportStats() {
	var (
		buffer = new(bytes.Buffer)
		stats  = bufio.NewWriter(buffer)
	)
	// Print some stats about the local broker
	fmt.Fprintf(stats, "Broker name:    %s\n", "TODO GET NAME")
	fmt.Fprintf(stats, "Broker address: %v\n", c.Addr)
	fmt.Fprintf(stats, "\n")

	// Print the members of the broker cluster
	brokers := make([]string, 0, len(c.Nodes))
	for name := range c.Nodes {
		brokers = append(brokers, name)
	}
	sort.Strings(brokers)

	c.ReportClusterMembers(stats, brokers)
	c.ReportConnectionMatrix(stats, brokers)
	c.ReportUnaccountedBrokers(stats, brokers)

	// Flush the entire stats to the console
	stats.Flush()
	//l.loggor.Info("Updated cluster topology\n\n" + buffer.String())
}

// reportClusterMembers creates a membership table to report which brokers the
// local node knows about and whether other nodes agree or not.
func (c *Cluster) ReportClusterMembers(w io.Writer, brokers []string) {
	fmt.Fprintf(w, "Cluster members:\n")

	members := make([][]string, 0, len(brokers))
	for i, broker := range brokers {
		var (
			id   = strconv.Itoa(i + 1)
			addr = c.Nodes[broker].String()
			age  = time.Since(time.Unix(0, int64(c.Times[broker]))).String()

			agree    = make([]string, 0, len(brokers))
			disagree = make([]string, 0, len(brokers))
			unknown  = make([]string, 0, len(brokers))
		)
		for j, name := range brokers {
			if c.Views[name][broker] == nil {
				unknown = append(unknown, strconv.Itoa(j+1))
			} else if c.Views[name][broker].Address != addr {
				disagree = append(disagree, strconv.Itoa(j+1))
			} else {
				agree = append(agree, strconv.Itoa(j+1))
			}
		}
		members = append(members, []string{id, broker, addr, age,
			fmt.Sprintf("%v", agree), fmt.Sprintf("%v", disagree), fmt.Sprintf("%v", unknown)})
	}
	table := tablewriter.NewWriter(w)
	table.SetHeader([]string{"#", "Name", "Address", "Updated", "Agree", "Disagree", "Unaware"})
	table.SetCenterSeparator("")
	table.SetColumnSeparator("")
	table.SetRowSeparator("")
	table.SetHeaderLine(false)
	table.SetBorder(false)
	table.AppendBulk(members)
	table.Render()

	fmt.Fprintf(w, "\n")
}

// reportConnectionMatrix creates a connection matrix to report on which broker
// reports being connected to which other brokers.
func (c *Cluster) ReportConnectionMatrix(w io.Writer, brokers []string) {
	fmt.Fprintf(w, "Connection matrix:\n")

	header := make([]string, 0, len(brokers))
	matrix := make([][]string, 0, len(brokers))

	for i, broker := range brokers {
		connected := make(map[int]bool)
		for name, node := range c.Views[broker] {
			if idx := sort.SearchStrings(brokers, name); idx < len(brokers) && brokers[idx] == name {
				connected[idx] = node.Alive
			}
		}
		row := []string{strconv.Itoa(i + 1)}
		for idx := 0; idx < len(brokers); idx++ {
			if connected[idx] {
				row = append(row, "Y")
			} else {
				row = append(row, "N")
			}
		}
		header = append(header, strconv.Itoa(i+1))
		matrix = append(matrix, row)
	}
	table := tablewriter.NewWriter(w)
	table.SetHeader(append([]string{""}, header...))
	table.SetCenterSeparator("")
	table.SetColumnSeparator("")
	table.SetRowSeparator("")
	table.SetHeaderLine(false)
	table.SetBorder(false)
	table.AppendBulk(matrix)
	table.Render()

	fmt.Fprintf(w, "\n")
}

// reportUnaccountedBrokers creates a report on brokers that remove nodes have
// advertised, but for some reason the local node rejected them.
func (c *Cluster) ReportUnaccountedBrokers(w io.Writer, brokers []string) {
	unaccounted := make([][]string, 0, len(brokers))
	for src, view := range c.Views {
		var extras []string
		for dst := range view {
			if idx := sort.SearchStrings(brokers, dst); idx == len(brokers) || brokers[idx] != dst {
				extras = append(extras, dst)
			}
		}
		if len(extras) > 0 {
			sort.Strings(extras)
			unaccounted = append(unaccounted, append([]string{src}, extras[0], view[extras[0]].Address))
			for _, extra := range extras[1:] {
				unaccounted = append(unaccounted, append([]string{""}, extra, view[extra].Address))
			}
		}
	}
	if len(unaccounted) == 0 {
		return
	}
	fmt.Fprintf(w, "Dangling views:\n")

	table := tablewriter.NewWriter(w)
	table.SetHeader(append([]string{"Source", "Target", "Address"}))
	table.SetCenterSeparator("")
	table.SetColumnSeparator("")
	table.SetRowSeparator("")
	table.SetHeaderLine(false)
	table.SetBorder(false)
	table.AppendBulk(unaccounted)
	table.Render()

	fmt.Fprintf(w, "\n")
}
