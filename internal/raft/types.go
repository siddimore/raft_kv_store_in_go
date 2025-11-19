package raft

import (
	"fmt"
	"log"
	"math/rand"
	"net/rpc"
	"sync"
	"time"
)

type NodeState int

const (
	Follower NodeState = iota
	Candidate
	Leader
)

// Log Entry represents a single log entry in the Raft consensus algorithm.
type LogEntry struct {
	Term    int         // Term when entry was received by leader
	Command interface{} // Command for state machine
}

// ApplyMsg represents a message to be applied to the state machine.
type ApplyMsg struct {
	Index   int         // Index of the log entry
	Command interface{} // Command to apply to state machine
}

// Raft Node is a single Raft Node in the cluster
type RaftNode struct {
	mu         sync.Mutex    // Mutex to protect shared access to this node's state
	id         int           // Node ID
	peers      []string      // List of peer node addresses
	rpcClients []*rpc.Client // outgoing RPC clients

	state       NodeState  // Current state of the node (Follower, Candidate, Leader)
	currentTerm int        // Latest term server has seen
	votedFor    *int       // CandidateId that received vote in current term
	log         []LogEntry // Log entries

	commitIndex int // Index of highest log entry known to be committed
	lastApplied int // Index of highest log entry applied to state machine

	nextIndex  []int // For each server, index of the next log entry to send to that server
	matchIndex []int // For each server, index of highest log entry known to be replicated on server

	applyCh         chan ApplyMsg // Channel to send applied messages to the state machine
	resetElectionCh chan struct{} // channel to reset election timer
	stopCh          chan struct{} // channel to stop the node

	electionTimer  *time.Timer // Timer for election timeout
	heartbeatTimer *time.Timer // Timer for sending heartbeats

	// For now, we'll embed a simple KV store directly.
	// Later we can separate this into internal/kv.
	kvStore map[string]string
}

// NewRaftNode creates and initializes a new Raft node
func NewRaftNode(id int, peers []string) *RaftNode {
	rn := &RaftNode{
		id:              id,
		peers:           peers,
		state:           Follower,
		log:             make([]LogEntry, 0),
		applyCh:         make(chan ApplyMsg, 64),
		resetElectionCh: make(chan struct{}, 1), // buffered channel length 1 to avoid blocking and reset timer
		stopCh:          make(chan struct{}),    // channel to stop the node
		kvStore:         make(map[string]string),
	}

	rn.rpcClients = make([]*rpc.Client, len(peers))

	rn.rpcClients = make([]*rpc.Client, len(peers))

	for i, addr := range peers {
		cli, err := rpc.Dial("tcp", addr)
		if err != nil {
			log.Printf("Node %d: failed to connect to peer %s: %v", id, addr, err)
			rn.rpcClients[i] = nil
		} else {
			rn.rpcClients[i] = cli
		}
	}

	rn.resetElectionTimer()
	go rn.run()

	return rn
}

func (rn *RaftNode) resetElectionTimer() {
	timeout := time.Duration(150+rand.Intn(150)) * time.Millisecond
	if rn.electionTimer == nil {
		rn.electionTimer = time.NewTimer(timeout)
		return
	}

	if !rn.electionTimer.Stop() {
		select {
		case <-rn.electionTimer.C: // drain channel
		default:
		}
	}

	rn.electionTimer.Reset(timeout)
}

// Event Loop for the Raft Node
func (rn *RaftNode) run() {
	for {
		select {
		case <-rn.stopCh:
			return // avoid goroutine leak
		case <-rn.electionTimer.C:
			rn.handleElectionTimeout()
		case <-rn.resetElectionCh:
			rn.resetElectionTimer()
		}
	}
}

func (rn *RaftNode) handleElectionTimeout() {
	rn.mu.Lock()
	// defer rn.mu.Unlock()

	// Become candidate
	rn.state = Candidate
	rn.currentTerm++
	rn.votedFor = &rn.id

	term := rn.currentTerm
	rn.resetElectionTimer()
	//peers := append([]string{}, rn.peers...) // copy
	rn.mu.Unlock()
	fmt.Println("Node", rn.id, "starting election for term", term)

	go rn.startElection()
	//go rn.startElection(term, peers)

}

func (rn *RaftNode) startElection() {
	rn.mu.Lock()

	rn.state = Candidate
	rn.currentTerm++
	term := rn.currentTerm
	rn.votedFor = &rn.id
	votes := 1

	peers := rn.peers
	majority := len(peers)/2 + 1

	log.Printf("Node %d starting election for term %d", rn.id, term)

	rn.resetElectionTimer()
	rn.mu.Unlock()

	var muVotes sync.Mutex

	for _, peer := range peers {
		peerAddr := peer

		go func(peerAddr string) {
			args := &RequestVoteArgs{
				Term:        term,
				CandidateID: rn.id,
				LastlogIndex: func() int {
					rn.mu.Lock()
					defer rn.mu.Unlock()
					return len(rn.log) - 1
				}(),
				LastlogTerm: func() int {
					rn.mu.Lock()
					defer rn.mu.Unlock()
					if len(rn.log) == 0 {
						return 0
					}
					return rn.log[len(rn.log)-1].Term
				}(),
			}

			var reply RequestVoteReply

			client, err := rpc.Dial("tcp", peerAddr)
			if err != nil {
				return
			}
			defer client.Close()

			if client.Call("RaftNode.RequestVote", args, &reply) != nil {
				return
			}

			rn.mu.Lock()
			defer rn.mu.Unlock()

			// Out-of-date term → become follower
			if reply.Term > rn.currentTerm {
				rn.currentTerm = reply.Term
				rn.state = Follower
				rn.votedFor = nil
				rn.resetElectionTimer()
				return
			}

			// Only count votes if still candidate for this term
			if rn.state != Candidate || rn.currentTerm != term {
				return
			}

			if reply.VoteGranted {
				muVotes.Lock()
				votes++
				v := votes
				muVotes.Unlock()

				if v >= majority {
					// We won! Convert to Leader
					rn.becomeLeader(term)
				}
			}
		}(peerAddr)
	}
}

func (rn *RaftNode) becomeLeader(term int) {
	// we will just return incase state of node
	// is already leader or term mismatch
	if rn.state != Candidate || rn.currentTerm != term {
		return
	}

	rn.state = Leader
	fmt.Println("Node", rn.id, "became LEADER for term", term)

	// Leader initializes nextIndex and matchIndex
	lastLogIndex := len(rn.log)
	rn.nextIndex = make([]int, len(rn.peers))
	rn.matchIndex = make([]int, len(rn.peers))
	for i := range rn.peers {
		rn.nextIndex[i] = lastLogIndex
		rn.matchIndex[i] = -1
	}
	rn.startHeartbeat()
}

func (rn *RaftNode) startHeartbeat() {
	rn.heartbeatTimer = (*time.Timer)(time.NewTicker(50 * time.Millisecond))
	// Send heartBeat whenever tick fires every 50 milliseconds
	// This will run infiintely until cancelled
	go func() {
		for {
			select {
			case <-rn.stopCh:
				return

			case <-rn.heartbeatTimer.C:
				rn.sendHeartbeats()
			}
		}
	}()
}

func (rn *RaftNode) sendHeartbeats() {
	rn.mu.Lock()
	// Leader sends heartbeat
	if rn.state != Leader {
		rn.mu.Unlock()
		return
	}
	term := rn.currentTerm
	// make a copy
	peers := append([]string{}, rn.peers...)
	rn.mu.Unlock()

	for _, peer := range peers {
		go func(peerAddr string) {
			args := &AppendEntriesArgs{
				Term:         term,
				LeaderID:     rn.id,
				PrevLogIndex: -1,
				PrevLogTerm:  -1,
				Entries:      nil,
				LeaderCommit: 0,
			}

			var reply AppendEntriesReply

			client, err := rpc.Dial("tcp", peerAddr)
			if err != nil {
				return
			}
			defer client.Close()

			_ = client.Call("RaftNode.AppendEntries", args, &reply)

		}(peer)
	}
}

// RequestVote RPC handler
func (rn *RaftNode) RequestVote(args *RequestVoteArgs, reply *RequestVoteReply) error {
	rn.mu.Lock()
	defer rn.mu.Unlock()

	// Initialize reply with current term
	reply.Term = rn.currentTerm
	reply.VoteGranted = false

	// If candidate's term is less than current term, reject
	if args.Term < rn.currentTerm {
		return nil
	}

	// If candidate's term is greater, update current term and convert to follower
	if args.Term > rn.currentTerm {
		rn.currentTerm = args.Term
		rn.votedFor = nil
		rn.state = Follower
	}

	// Grant vote if we haven't voted yet or already voted for this candidate
	if rn.votedFor == nil || *rn.votedFor == args.CandidateID {
		// TODO: Add log comparison logic
		lastLogIndex := len(rn.log) - 1
		lastLogTerm := 0
		if lastLogIndex >= 0 {
			lastLogTerm = rn.log[lastLogIndex].Term
		}
		upToDate :=
			args.LastlogTerm > lastLogTerm ||
				(args.LastlogTerm == lastLogTerm && args.LastlogIndex >= lastLogIndex)
		if upToDate {
			rn.votedFor = &args.CandidateID
			reply.VoteGranted = true
			// Reset election timeout when voting (Raft rule)
			select {
			case rn.resetElectionCh <- struct{}{}:
			default:
			}
		}
	}

	return nil
}

// AppendEntries RPC handler
func (rn *RaftNode) AppendEntries(args *AppendEntriesArgs, reply *AppendEntriesReply) error {
	rn.mu.Lock()
	defer rn.mu.Unlock()

	// Initialize reply with current term
	reply.Term = rn.currentTerm
	reply.Success = false

	// If leader's term is less than current term, reject
	if args.Term < rn.currentTerm {
		return nil
	}

	// If leader's term is greater or equal, update current term and convert to follower
	if args.Term >= rn.currentTerm {
		rn.currentTerm = args.Term
		rn.state = Follower
	}

	// TODO: Implement full AppendEntries logic
	reply.Success = true

	return nil
}
