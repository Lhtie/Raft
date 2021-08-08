package raft

//
// this is an outline of the API that raft must expose to
// the service (or tester). see comments below for
// each of these functions for more details.
//
// rf = Make(...)
//   create a new Raft server.
// rf.Start(command interface{}) (index, term, isleader)
//   start agreement on a new log entry
// rf.GetState() (term, isLeader)
//   ask a Raft for its current term, and whether it thinks it is leader
// ApplyMsg
//   each time a new entry is committed to the log, each Raft peer
//   should send an ApplyMsg to the service (or tester)
//   in the same server.
//

import (
	"Raft/labgob"
	"bytes"
	"math/rand"
	"sync"
	"time"
)
import "Raft/labrpc"

// import "bytes"
// import "labgob"

const(
	Follower = iota
	Candidate
	Leader
)

//
// as each Raft peer becomes aware that successive log entries are
// committed, the peer should send an ApplyMsg to the service (or
// tester) on the same server, via the applyCh passed to Make(). set
// CommandValid to true to indicate that the ApplyMsg contains a newly
// committed log entry.
//
// in Lab 3 you'll want to send other kinds of messages (e.g.,
// snapshots) on the applyCh; at that point you can add fields to
// ApplyMsg, but set CommandValid to false for these other uses.
//
type ApplyMsg struct {
	CommandValid bool
	Command      interface{}
	CommandIndex int
	Snapshot	 []byte
}

//
// A Go object implementing a single Raft peer.
//
type Raft struct {
	mu        sync.Mutex          // Lock to protect shared access to this peer's state
	peers     []*labrpc.ClientEnd // RPC end points of all peers
	persister *Persister          // Object to hold this peer's persisted state
	me        int                 // this peer's index into peers[]

	// Your data here (2A, 2B, 2C).
	// Look at the paper's Figure 2 for a description of what
	// state a Raft server must maintain.

	state		int // Follower, Candidate, Leader
	applyCh		chan ApplyMsg

	// Persistent state on all servers
	currentTerm	int
	votedFor	int
	log			[]LogEntry

	// Persistent part of snapshot on all servers
	lastIncludedIndex	int
	lastIncludedTerm	int

	// Volatile state on all servers
	commitIndex int
	lastApplied int

	// Volatile state on leaders
	nextIndex	[]int
	matchIndex	[]int

	// Intervals
	heartbeatInterval int
	applyInterval	 int

	// Timers
	heartbeatTimer *time.Timer
	applyTimer	   *time.Timer
	electionTimer  *time.Timer

}

type LogEntry struct{
	Command		 interface{}
	Term	 	 int
	LogIndex	 int
}

func min(l, r int) int{
	if l < r{
		return l
	} else {
		return r
	}
}

// return currentTerm and whether this server
// believes it is the leader.
func (rf *Raft) GetState() (int, bool) {
	var term int
	var isleader bool
	// Your code here (2A).
	rf.mu.Lock()
	term = rf.currentTerm
	isleader = rf.state == Leader
	rf.mu.Unlock()
	return term, isleader
}

func (rf *Raft) GetPersister() (*Persister){
	rf.mu.Lock()
	defer rf.mu.Unlock()

	return rf.persister
}

//
// save Raft's persistent state to stable storage,
// where it can later be retrieved after a crash and restart.
// see paper's Figure 2 for a description of what should be persistent.
//
func (rf *Raft) persist() {
	// Your code here (2C).
	w := new(bytes.Buffer)
	e := labgob.NewEncoder(w)
	e.Encode(rf.currentTerm)
	e.Encode(rf.votedFor)
	e.Encode(rf.log)
	e.Encode(rf.lastIncludedIndex)
	e.Encode(rf.lastIncludedTerm)
	data := w.Bytes()
	rf.persister.SaveRaftState(data)
}

func (rf *Raft) persistStateAndSnapshot(snapshot []byte) {
	w := new(bytes.Buffer)
	e := labgob.NewEncoder(w)
	e.Encode(rf.currentTerm)
	e.Encode(rf.votedFor)
	e.Encode(rf.log)
	e.Encode(rf.lastIncludedIndex)
	e.Encode(rf.lastIncludedTerm)
	data := w.Bytes()
	rf.persister.SaveStateAndSnapshot(data, snapshot)
}


//
// restore previously persisted state.
//
func (rf *Raft) readPersist(data []byte) bool {
	if data == nil || len(data) < 1 { // bootstrap without any state?
		return false
	}
	//Your code here (2C).
	rf.mu.Lock()
	defer rf.mu.Unlock()

	r := bytes.NewBuffer(data)
	d := labgob.NewDecoder(r)
	var currentTerm, votedFor, lastIncludedIndex, lastIncludedTerm int
	var log []LogEntry
	if d.Decode(&currentTerm) == nil{
		rf.currentTerm = currentTerm
	} else {
		return false
	}
	if d.Decode(&votedFor) == nil{
		rf.votedFor = votedFor
	} else {
		return false
	}
	if d.Decode(&log) == nil{
		rf.log = log
	} else {
		return false
	}
	if d.Decode(&lastIncludedIndex) == nil{
		rf.lastIncludedIndex = lastIncludedIndex
	} else {
		return false
	}
	if d.Decode(&lastIncludedTerm) == nil{
		rf.lastIncludedTerm = lastIncludedTerm
	} else {
		return false
	}
	return true
}

func (rf *Raft) ResizeLogEntries(index int, snapshot []byte){
	rf.mu.Lock()
	defer rf.mu.Unlock()

	if index <= rf.lastIncludedIndex { return }
	i := 1
	for ; i < len(rf.log); i++{
		if rf.log[i].LogIndex == index { break }
	}
	if i == len(rf.log) { return }
	rf.lastIncludedIndex = index
	rf.lastIncludedTerm = rf.log[i].Term
	rf.log = rf.log[i:len(rf.log)]
	rf.persistStateAndSnapshot(snapshot)
}

func (rf *Raft) becomeFollower(term int){ // concurrent insecure
	rf.state = Follower
	rf.electionTimer.Reset(getElectionDuration())
	if term > rf.currentTerm {
		rf.currentTerm = term
		rf.votedFor = -1
		rf.persist()
	}
}

func (rf *Raft) becomeCandidate(){ // concurrent secure
	rf.mu.Lock()
	rf.state = Candidate
	rf.mu.Unlock()
	rf.startElection()
}

func (rf *Raft) becomeLeader() { // concurrent secure
	rf.mu.Lock()
	rf.state = Leader
	rf.nextIndex = make([]int, len(rf.peers))
	rf.matchIndex = make([]int, len(rf.peers))
	for id := range rf.peers {
		rf.nextIndex[id] = rf.lastIncludedIndex + len(rf.log)
		rf.matchIndex[id] = rf.lastIncludedIndex
	}
	rf.mu.Unlock()
	go rf.pingLoop()
}

func (rf *Raft) applyLoop(){
	rf.mu.Lock()
	rf.applyTimer = time.NewTimer(time.Duration(rf.applyInterval) * time.Millisecond)
	rf.mu.Unlock()
	for {
		<-rf.applyTimer.C
		rf.mu.Lock()
		for rf.lastApplied < rf.commitIndex{
			rf.lastApplied++
			msg := ApplyMsg{true, rf.log[rf.lastApplied-rf.lastIncludedIndex].Command, rf.lastApplied, nil}
			rf.mu.Unlock()
			rf.applyCh <- msg
			rf.mu.Lock()
		}
		rf.applyTimer.Reset(time.Duration(rf.applyInterval) * time.Millisecond)
		rf.mu.Unlock()
	}
}

//
// example RequestVote RPC arguments structure.
// field names must start with capital letters!
//
type RequestVoteArgs struct {
	// Your data here (2A, 2B).
	Term		 int
	CandidateId	 int
	LastLogIndex int
	LastLogTerm	 int
}

//
// example RequestVote RPC reply structure.
// field names must start with capital letters!
//
type RequestVoteReply struct {
	// Your data here (2A).
	Term		int
	VoteGranted	bool
}

//
// example RequestVote RPC handler.
//
func (rf *Raft) RequestVote(args *RequestVoteArgs, reply *RequestVoteReply) {
	// Your code here (2A, 2B).
	rf.mu.Lock()
	defer rf.mu.Unlock()
	if args.Term > rf.currentTerm{
		rf.becomeFollower(args.Term)
	}
	reply.Term = rf.currentTerm
	if args.Term < rf.currentTerm{
		reply.VoteGranted = false
		return
	}
	if rf.votedFor == -1 || rf.votedFor == args.CandidateId{
		LastLogIndex := rf.lastIncludedIndex + len(rf.log) - 1
		LastLogTerm := rf.log[len(rf.log)-1].Term
		if args.LastLogTerm > LastLogTerm || (args.LastLogTerm == LastLogTerm && args.LastLogIndex >= LastLogIndex){
			reply.VoteGranted = true
			rf.votedFor = args.CandidateId
			rf.persist()
			rf.electionTimer.Reset(getElectionDuration())
		} else {
			reply.VoteGranted = false
		}
	} else {
		reply.VoteGranted = false
	}
}

//
// example code to send a RequestVote RPC to a server.
// server is the index of the target server in rf.peers[].
// expects RPC arguments in args.
// fills in *reply with RPC reply, so caller should
// pass &reply.
// the types of the args and reply passed to Call() must be
// the same as the types of the arguments declared in the
// handler function (including whether they are pointers).
//
// The labrpc package simulates a lossy network, in which servers
// may be unreachable, and in which requests and replies may be lost.
// Call() sends a request and waits for a reply. If a reply arrives
// within a timeout interval, Call() returns true; otherwise
// Call() returns false. Thus Call() may not return for a while.
// A false return can be caused by a dead server, a live server that
// can't be reached, a lost request, or a lost reply.
//
// Call() is guaranteed to return (perhaps after a delay) *except* if the
// handler function on the server side does not return.  Thus there
// is no need to implement your own timeouts around Call().
//
// look at the comments in ../labrpc/labrpc.go for more details.
//
// if you're having trouble getting RPC to work, check that you've
// capitalized all field names in structs passed over RPC, and
// that the caller passes the address of the reply struct with &, not
// the struct itself.
//
func (rf *Raft) sendRequestVote(server int, args *RequestVoteArgs, reply *RequestVoteReply) bool {
	ok := rf.peers[server].Call("Raft.RequestVote", args, reply)
	return ok
}

func getElectionDuration() time.Duration{
	electionInterval := rand.Intn(150) + 150
	return time.Duration(electionInterval) * time.Millisecond
}

func (rf *Raft) startElection(){
	rf.mu.Lock()
	rf.currentTerm++
	rf.votedFor = rf.me
	last := rf.lastIncludedIndex + len(rf.log) - 1
	rf.persist()
	args := RequestVoteArgs{rf.currentTerm, rf.me, last, rf.log[len(rf.log)-1].Term}
	rf.electionTimer.Reset(getElectionDuration())
	rf.mu.Unlock()
	succ := 1
	for id := range rf.peers{
		if id == rf.me {continue}
		go func(id int){
			var reply RequestVoteReply
			res := rf.sendRequestVote(id, &args, &reply)
			if res{
				rf.mu.Lock()
				if args.Term < rf.currentTerm {
					rf.mu.Unlock()
					return
				}
				if reply.Term > rf.currentTerm{
					rf.becomeFollower(reply.Term)
				}
				if reply.VoteGranted {
					succ++
					if rf.state == Candidate && succ > len(rf.peers) / 2{
						rf.mu.Unlock()
						rf.becomeLeader()
						return
					}
				}
				rf.mu.Unlock()
			}
		} (id)
	}
}

func (rf *Raft) electionLoop(){
	rf.mu.Lock()
	rf.electionTimer = time.NewTimer(getElectionDuration())
	rf.mu.Unlock()
	for {
		<-rf.electionTimer.C
		rf.mu.Lock()
		if rf.state != Leader {
			rf.mu.Unlock()
			rf.becomeCandidate()
			continue
		} else {
			rf.electionTimer.Reset(getElectionDuration())
		}
		rf.mu.Unlock()
	}
}

type AppendEntriesArgs struct {
	Term			int
	LeaderId		int
	PrevLogIndex	int
	PrevLogTerm		int
	Entries			[]LogEntry
	LeaderCommit	int
}

type AppendEntriesReply struct {
	Term		int
	NextIndex	int
	Success 	bool
}

func (rf *Raft) AppendEntries(args *AppendEntriesArgs, reply *AppendEntriesReply) {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	rf.electionTimer.Reset(getElectionDuration())
	if args.Term >= rf.currentTerm {
		rf.becomeFollower(args.Term)
	}
	reply.Term = rf.currentTerm
	if args.Term < rf.currentTerm{
		reply.Success = false
		return
	}
	if rf.lastIncludedIndex + len(rf.log) - 1 < args.PrevLogIndex{
		reply.Success = false
		reply.NextIndex = rf.lastIncludedIndex + len(rf.log)
		return
	}
	if args.PrevLogIndex - rf.lastIncludedIndex < 0{
		reply.Success = false
		reply.NextIndex = rf.lastIncludedIndex + 1
		return
	}
	if rf.log[args.PrevLogIndex-rf.lastIncludedIndex].Term != args.PrevLogTerm{
		reply.Success = false
		reply.NextIndex = args.PrevLogIndex
		for ; reply.NextIndex > rf.lastIncludedIndex + 1 && rf.log[reply.NextIndex-rf.lastIncludedIndex-1].Term == rf.log[args.PrevLogIndex-rf.lastIncludedIndex].Term; reply.NextIndex-- {}
		return
	}
	reply.Success = true
	i, j := args.PrevLogIndex + 1, 0
	for i < rf.lastIncludedIndex + len(rf.log) && j < len(args.Entries) {
		if rf.log[i-rf.lastIncludedIndex].Term != args.Entries[j].Term {break}
		i += 1; j += 1
	}
	if j < len(args.Entries) {rf.log = rf.log[:i-rf.lastIncludedIndex]}
	for j < len(args.Entries){
		rf.log = append(rf.log, args.Entries[j])
		i += 1; j += 1
	}
	rf.persist()
	if args.LeaderCommit > rf.commitIndex{
		rf.commitIndex = min(args.LeaderCommit, rf.lastIncludedIndex + len(rf.log) - 1)
	}
}

func (rf *Raft) sendAppendEntries(server int, args *AppendEntriesArgs, reply *AppendEntriesReply) bool {
	ok := rf.peers[server].Call("Raft.AppendEntries", args, reply)
	return ok
}

type InstallSnapshotArgs struct {
	Term				int
	LeaderId			int
	LastIncludedIndex	int
	LastIncludedTerm	int
	Data				[]byte
}

type InstallSnapshotReply struct {
	Term	int
}

func (rf *Raft) InstallSnapshot(args *InstallSnapshotArgs, reply *InstallSnapshotReply){
	rf.mu.Lock()
	rf.electionTimer.Reset(getElectionDuration())
	if args.Term >= rf.currentTerm {
		rf.becomeFollower(args.Term)
	}
	reply.Term = rf.currentTerm
	if args.Term < rf.currentTerm{
		rf.mu.Unlock()
		return
	}
	if rf.commitIndex >= args.LastIncludedIndex{
		rf.mu.Unlock()
		return
	}
	rf.lastIncludedIndex = args.LastIncludedIndex
	rf.lastApplied = args.LastIncludedIndex
	rf.commitIndex = args.LastIncludedIndex
	rf.lastIncludedTerm = args.LastIncludedTerm
	rf.log = make([]LogEntry, 1)
	rf.log[0].LogIndex = rf.lastIncludedIndex
	rf.log[0].Term = rf.lastIncludedTerm
	rf.persistStateAndSnapshot(args.Data)
	rf.mu.Unlock()
	msg := ApplyMsg{false, nil, 0, args.Data}
	rf.applyCh <- msg
}

func (rf *Raft) sendInstallSnapshot(server int, args *InstallSnapshotArgs, reply *InstallSnapshotReply) bool {
	ok := rf.peers[server].Call("Raft.InstallSnapshot", args, reply)
	return ok
}

func (rf *Raft) canCommit(N int) bool{
	cnt := 0
	for id := range rf.peers{
		if rf.matchIndex[id] >= N{
			cnt++
		}
	}
	return cnt > len(rf.peers) / 2
}

func (rf *Raft) pingLoop(){
	rf.mu.Lock()
	rf.heartbeatTimer = time.NewTimer(0)
	rf.mu.Unlock()
	for {
		<-rf.heartbeatTimer.C
		rf.mu.Lock()
		if rf.state != Leader {
			rf.mu.Unlock()
			return
		}
		args_append := make([]AppendEntriesArgs, len(rf.peers))
		args_snapshot := make([]InstallSnapshotArgs, len(rf.peers))
		types := make([]string, len(rf.peers))
		for id := range rf.peers{
			if id == rf.me {continue}
			if rf.nextIndex[id] <= rf.lastIncludedIndex{
				types[id] = "InstallSnapshot"
				args_snapshot[id] = InstallSnapshotArgs{rf.currentTerm, rf.me, rf.lastIncludedIndex, rf.lastIncludedTerm, rf.persister.snapshot}
			} else {
				types[id] = "AppendEntries"
				args_append[id].Term = rf.currentTerm
				args_append[id].LeaderId = rf.me
				args_append[id].PrevLogIndex = rf.nextIndex[id] - 1
				args_append[id].PrevLogTerm = rf.log[rf.nextIndex[id]-1-rf.lastIncludedIndex].Term
				args_append[id].LeaderCommit = rf.commitIndex
				args_append[id].Entries = make([]LogEntry, 0)
				if rf.lastIncludedIndex + len(rf.log) - 1 >= rf.nextIndex[id] {
					for i := rf.nextIndex[id]; i < rf.lastIncludedIndex + len(rf.log); i++ {
						args_append[id].Entries = append(args_append[id].Entries, rf.log[i-rf.lastIncludedIndex])
					}
				}
			}
		}
		rf.heartbeatTimer.Reset(time.Duration(rf.heartbeatInterval) * time.Millisecond)
		rf.mu.Unlock()
		for id := range rf.peers{
			if id == rf.me {
				rf.mu.Lock()
				rf.nextIndex[id] = rf.lastIncludedIndex + len(rf.log)
				rf.matchIndex[id] = rf.lastIncludedIndex + len(rf.log) - 1
				rf.mu.Unlock()
				continue
			}
			if types[id] == "AppendEntries" {
				go func(id int) {
					var reply AppendEntriesReply
					res := rf.sendAppendEntries(id, &args_append[id], &reply)
					if res {
						rf.mu.Lock()
						if args_append[id].Term < rf.currentTerm {
							rf.mu.Unlock()
							return
						}
						if reply.Term > rf.currentTerm {
							rf.becomeFollower(reply.Term)
						}
						if reply.Success {
							rf.nextIndex[id] = rf.lastIncludedIndex + len(rf.log)
							rf.matchIndex[id] = args_append[id].PrevLogIndex + len(args_append[id].Entries)
							newCommit := rf.commitIndex
							for ; newCommit + 1 < rf.lastIncludedIndex + len(rf.log) && rf.canCommit(newCommit + 1); newCommit++ {}
							if newCommit > rf.commitIndex && rf.log[newCommit-rf.lastIncludedIndex].Term == rf.currentTerm {
								rf.commitIndex = newCommit
							}
						} else {
							if reply.Term == args_append[id].Term {
								rf.nextIndex[id] = reply.NextIndex
							}
						}
						rf.mu.Unlock()
					}
				}(id)
			} else if types[id] == "InstallSnapshot" {
				go func(id int){
					var reply InstallSnapshotReply
					ok := rf.sendInstallSnapshot(id, &args_snapshot[id], &reply)
					if ok{
						rf.mu.Lock()
						if args_snapshot[id].Term < rf.currentTerm {
							rf.mu.Unlock()
							return
						}
						if reply.Term > rf.currentTerm{
							rf.becomeFollower(reply.Term)
						} else {
							rf.nextIndex[id] = args_snapshot[id].LastIncludedIndex + 1
							rf.matchIndex[id] = args_snapshot[id].LastIncludedIndex
						}
						rf.mu.Unlock()
					}
				}(id)
			}
		}
	}
}

//
// the service using Raft (e.g. a k/v server) wants to start
// agreement on the next command to be appended to Raft's log. if this
// server isn't the leader, returns false. otherwise start the
// agreement and return immediately. there is no guarantee that this
// command will ever be committed to the Raft log, since the leader
// may fail or lose an election. even if the Raft instance has been killed,
// this function should return gracefully.
//
// the first return value is the index that the command will appear at
// if it's ever committed. the second return value is the current
// term. the third return value is true if this server believes it is
// the leader.
//
func (rf *Raft) Start(command interface{}) (int, int, bool) {
	index := -1
	term := -1
	isLeader := true

	// Your code here (2B).
	rf.mu.Lock()
	if rf.state != Leader{
		rf.mu.Unlock()
		isLeader = false
		return index, term, isLeader
	}
	index = rf.lastIncludedIndex + len(rf.log)
	term = rf.currentTerm
	rf.log = append(rf.log, LogEntry{command, term, index})
	rf.persist()
	rf.heartbeatTimer.Reset(0)
	rf.mu.Unlock()

	return index, term, isLeader
}

//
// the tester calls Kill() when a Raft instance won't
// be needed again. you are not required to do anything
// in Kill(), but it might be convenient to (for example)
// turn off debug output from this instance.
//
func (rf *Raft) Kill() {
	// Your code here, if desired.
}

//
// the service or tester wants to create a Raft server. the ports
// of all the Raft servers (including this one) are in peers[]. this
// server's port is peers[me]. all the servers' peers[] arrays
// have the same order. persister is a place for this server to
// save its persistent state, and also initially holds the most
// recent saved state, if any. applyCh is a channel on which the
// tester or service expects Raft to send ApplyMsg messages.
// Make() must return quickly, so it should start goroutines
// for any long-running work.
//
func Make(peers []*labrpc.ClientEnd, me int,
	persister *Persister, applyCh chan ApplyMsg) *Raft {
	rf := &Raft{}
	rf.peers = peers
	rf.persister = persister
	rf.me = me

	// Your initialization code here (2A, 2B, 2C).

	// initialize from state persisted before a crash
	initialized := rf.readPersist(persister.ReadRaftState())

	rand.Seed(time.Now().Unix())
	rf.mu.Lock()

	rf.applyCh = applyCh
	rf.state = Follower

	if !initialized {
		rf.currentTerm = 0
		rf.votedFor = -1
		rf.log = make([]LogEntry, 1)
		rf.log[0].Term = 0
		rf.log[0].LogIndex = 0
		rf.lastIncludedIndex = 0
		rf.lastIncludedTerm = 0
	}

	rf.commitIndex = rf.lastIncludedIndex
	rf.lastApplied = rf.lastIncludedIndex

	rf.heartbeatInterval = 40
	rf.applyInterval = 10

	rf.mu.Unlock()
	go rf.electionLoop()
	go rf.applyLoop()

	return rf
}
