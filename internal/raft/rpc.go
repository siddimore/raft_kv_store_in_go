package raft

type AppendEntriesArgs struct {
	Term         int        // Leader’s term
	LeaderID     int        // So follower can redirect clients
	PrevLogIndex int        // Index of log entry immediately preceding new ones
	PrevLogTerm  int        // Term of PrevLogIndex entry
	Entries      []LogEntry // Log entries to store (empty for heartbeat; may send more than one for efficiency)
	LeaderCommit int        // Leader’s commitIndex
}

type AppendEntriesReply struct {
	Term    int  // Current term, for leader to update itself
	Success bool // True if follower contained entry matching PrevLogIndex and PrevLogTerm
}

type RequestVoteArgs struct {
	Term         int // Current term, for leader to update itself
	CandidateID  int // CandidateId to which vote is granted
	LastlogIndex int // Index of last log entry
	LastlogTerm  int // Term of last log entry
}

type RequestVoteReply struct {
	Term        int  // Current term, for candidate to update itself
	VoteGranted bool // True means candidate received vote
}
