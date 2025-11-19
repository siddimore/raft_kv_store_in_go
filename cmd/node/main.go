package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/rpc"

	"github.com/siddimore/raft-kv/internal/raft"
)

func main() {
	id := flag.Int("id", 0, "Node ID")
	addr := flag.String("addr", "localhost:8000", "Node address")
	flag.Parse()

	fmt.Printf("Starting node %d at %s\n", *id, *addr)

	raftNode := raft.NewRaftNode(*id, nil)
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
