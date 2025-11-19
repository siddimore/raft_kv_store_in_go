## 🛠️ Raft-Based Distributed Key-Value Store (Go)

This project is a from-scratch implementation of a distributed key-value store built on top of the Raft consensus algorithm.
It’s designed as a learning-oriented implementation that still mirrors the core mechanics of real Raft clusters: leader election, heartbeats, log replication, and applying committed state to a KV store.

## 🚀 Features Implemented So Far
✅ Core Raft Node Skeleton
RaftNode struct with complete internal state:
currentTerm, votedFor, log, commit index, last applied index, etc.
Follower / Candidate / Leader states (state machine evolving).

✅ Timers & Concurrency
Fully functional randomized election timeout using time.Timer.
Clean reset logic with resetElectionCh.
Background Raft control-plane goroutine (run()) that reacts to:
election timeouts
reset events
stop signals

✅ RPC Handlers
AppendEntries RPC (heartbeat only for now)
RequestVote RPC (partially implemented)
RPC server using Go's built-in net/rpc.

✅ Elections
Nodes start elections when they experience an election timeout.
Term increments correctly (starting election for term X log output).
Nodes request votes from peers when becoming a Candidate.
Node transitions:
Follower → Candidate → Leader (in-progress)

## ⏳ Coming Soon / Work in Progress

Log replication
Leader heartbeat broadcast
State machine applying committed entries
Client API for KV ops
Snapshotting 

## Architecture Overview
Raft Components Implemented:
```
RaftNode
 ├── Persistent state: currentTerm, votedFor, log[]
 ├── Volatile state: commitIndex, lastApplied
 ├── Leader state: nextIndex[], matchIndex[]
 ├── Timers:
 │     ├── electionTimer (randomized)
 │     └── heartbeatTimer (leader only)
 ├── Channels:
 │     ├── applyCh
 │     ├── resetElectionCh
 │     └── stopCh
 └── Goroutines:
       ├── main event loop (run)
       └── RPC handlers (AppendEntries, RequestVote)
```
## 🏗️ Running a Local Raft Cluster

Start multiple nodes as local processes.
```
Node 1
go run ./cmd/node -id=1 -addr=127.0.0.1:8001 -peers=127.0.0.1:8002,127.0.0.1:8003
Node 2
go run ./cmd/node -id=2 -addr=127.0.0.1:8002 -peers=127.0.0.1:8001,127.0.0.1:8003
Node 3
go run ./cmd/node -id=3 -addr=127.0.0.1:8003 -peers=127.0.0.1:8001,127.0.0.1:8002


Expected output:
Node 1 starting election for term 1
Node 1 starting election for term 2
...
Node 2 starting election for term 1
Node 3 starting election for term 1
Once RequestVote RPCs are fully wired, you’ll see:
Node X becomes leader for term Y
Sending heartbeats...
```

## 📦 Project Layout
```
├── cmd/
│   └── node/          # Raft node executable
├── internal/
│   ├── raft/          # Raft implementation
│   └── kv/            # KV store application layer
└── README.md
```

## 📚 Goals of This Project
This repo is intended to be:
A learning resource for understanding Raft in real cod
A clean and readable implementation of the algorithm

A foundation you can extend with:
```
storage backends
real client APIs
high availability behavior
snapshotting
persistent logs
```
