package raftkv

import (
	"Raft/labrpc"
	"sync"
	"time"
)
import "crypto/rand"
import "math/big"

type totalOpsType struct{
	mu	sync.Mutex
	cnt	int
}

var totalOps = totalOpsType{cnt: 0}

type Clerk struct {
	servers []*labrpc.ClientEnd
	// You will have to modify this struct.

	mu				sync.Mutex
	id 				int
	timeoutInterval	int
	timeoutTimer	*time.Timer
}

func nrand() int64 {
	max := big.NewInt(int64(1) << 62)
	bigx, _ := rand.Int(rand.Reader, max)
	x := bigx.Int64()
	return x
}

func MakeClerk(servers []*labrpc.ClientEnd) *Clerk {
	ck := new(Clerk)
	ck.servers = servers
	// You'll have to add code here.

	ck.id = -1
	ck.timeoutInterval = 50
	ck.timeoutTimer = time.NewTimer(0)
	return ck
}

//
// fetch the current value for a key.
// returns "" if the key does not exist.
// keeps trying forever in the face of all other errors.
//
// you can send an RPC with code like this:
// ok := ck.servers[i].Call("KVServer.Get", &args, &reply)
//
// the types of args and reply (including whether they are pointers)
// must match the declared types of the RPC handler function's
// arguments. and reply must be passed as a pointer.
//
func (ck *Clerk) Get(key string) string {
	// You will have to modify this function.
	totalOps.mu.Lock()
	args := GetArgs{key, totalOps.cnt}
	totalOps.cnt++
	totalOps.mu.Unlock()
	doneCh := make(chan GetReply)
	for {
		ck.mu.Lock()
		ck.timeoutTimer.Reset(time.Duration(ck.timeoutInterval) * time.Millisecond)
		ck.mu.Unlock()
		go func(){
			var reply GetReply
			ck.mu.Lock()
			if ck.id == -1 {ck.id = int(nrand() % int64(len(ck.servers))) }
			id := ck.id
			ck.mu.Unlock()
			ok := ck.servers[id].Call("KVServer.Get", &args, &reply)
			ck.mu.Lock()
			if ok{
				if reply.WrongLeader{
					ck.id = -1
				} else {
					ck.mu.Unlock()
					doneCh <- reply
					return
				}
			} else {
				ck.id = -1
			}
			ck.mu.Unlock()
		} ()
		select{
			case ans := <-doneCh:
				if ans.Err == OK {
					return ans.Value
				} else {
					return ""
				}
			default:
		}
		<-ck.timeoutTimer.C
	}
}

//
// shared by Put and Append.
//
// you can send an RPC with code like this:
// ok := ck.servers[i].Call("KVServer.PutAppend", &args, &reply)
//
// the types of args and reply (including whether they are pointers)
// must match the declared types of the RPC handler function's
// arguments. and reply must be passed as a pointer.
//
func (ck *Clerk) PutAppend(key string, value string, op string) {
	// You will have to modify this function.
	totalOps.mu.Lock()
	args := PutAppendArgs{key, value, op, totalOps.cnt}
	totalOps.cnt++
	totalOps.mu.Unlock()
	doneCh := make(chan bool)
	for {
		ck.mu.Lock()
		ck.timeoutTimer.Reset(time.Duration(ck.timeoutInterval) * time.Millisecond)
		ck.mu.Unlock()
		go func(){
			var reply PutAppendReply
			ck.mu.Lock()
			if ck.id == -1 {ck.id = int(nrand() % int64(len(ck.servers))) }
			id := ck.id
			ck.mu.Unlock()
			ok := ck.servers[id].Call("KVServer.PutAppend", &args, &reply)
			ck.mu.Lock()
			if ok{
				if reply.WrongLeader{
					ck.id = -1
				} else {
					ck.mu.Unlock()
					doneCh <- true
					return
				}
			} else {
				ck.id = -1
			}
			ck.mu.Unlock()
		} ()
		select{
			case <-doneCh: return
			default:
		}
		<-ck.timeoutTimer.C
	}
}

func (ck *Clerk) Put(key string, value string) {
	ck.PutAppend(key, value, "Put")
}
func (ck *Clerk) Append(key string, value string) {
	ck.PutAppend(key, value, "Append")
}
