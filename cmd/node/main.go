package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/rpc"
	"strings"

	"github.com/siddimore/raft-kv/internal/raft"
)

func main() {
	id := flag.Int("id", 0, "Node ID")
	addr := flag.String("addr", "127.0.0.1:8000", "Node address")
	peersStr := flag.String("peers", "", "Comma-separated list of peer addresses")

	flag.Parse()

	var peers []string
	if *peersStr != "" {
		peers = strings.Split(*peersStr, ",")
	}

	fmt.Printf("Starting node %d at %s with peers: %v\n", *id, *addr, peers)

	raftNode := raft.NewRaftNode(*id, peers)
	server := rpc.NewServer()
	if err := server.RegisterName("RaftNode", raftNode); err != nil {
		log.Fatalf("failed to register RPC: %v", err)
	}

	l, err := net.Listen("tcp", *addr)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	defer l.Close()

	log.Printf("Node %d listening on %s", *id, *addr)

	for {
		conn, err := l.Accept()
		if err != nil {
			log.Printf("accept error: %v", err)
			continue
		}
		go server.ServeConn(conn)
	}
}
