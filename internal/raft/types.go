package raft

import (
	"fmt"
	"math/rand"
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
	mu    sync.Mutex // Mutex to protect shared access to this node's state
	id    int        // Node ID
	peers []string   // List of peer node addresses

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
	defer rn.mu.Unlock()

	fmt.Println("Node", rn.id, "Election TimeOut Fired")

	rn.resetElectionTimer()
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
		rn.votedFor = &args.CandidateID
		reply.VoteGranted = true
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
