package raftkv

import (
	"Raft/labgob"
	"Raft/labrpc"
	"Raft/raft"
	"bytes"
	"log"
	"sync"
)

const Debug = 0

func DPrintf(format string, a ...interface{}) (n int, err error) {
	if Debug > 0 {
		log.Printf(format, a...)
	}
	return
}

const(Get_op = iota; Put_op; Append_op)

type Op struct {
	// Your definitions here.
	// Field names must start with capital letters,
	// otherwise RPC will break.
	Type	int
	ClerkId int
	Id		int
	Key		string
	Value	string
	Err		Err
	DoneCh	chan Result
}

type Result struct{
	Prepared	bool
	Value		string
	Err			Err
}

type KVServer struct {
	mu      sync.Mutex
	me      int
	rf      *raft.Raft
	applyCh chan raft.ApplyMsg

	maxraftstate int // snapshot if log grows this big

	// Your definitions here.

	data		map[string]string
	lastOpId	map[int]int
	processed	map[int]Result
}

func (kv *KVServer) snapshot(logIndex int){
	w := new(bytes.Buffer)
	e := labgob.NewEncoder(w)
	e.Encode(kv.data)
	e.Encode(kv.lastOpId)
	e.Encode(kv.processed)
	data := w.Bytes()
	kv.rf.ResizeLogEntries(logIndex, data)
}

func (kv *KVServer) readSnapshot(snapshot []byte) bool{
	if snapshot == nil || len(snapshot) < 1 { // bootstrap without any state?
		return false
	}
	kv.mu.Lock()
	defer kv.mu.Unlock()

	r := bytes.NewBuffer(snapshot)
	d := labgob.NewDecoder(r)
	var data map[string]string
	var lastOpId map[int]int
	var processed map[int]Result
	if d.Decode(&data) == nil{
		kv.data = data
	} else {
		return false
	}
	if d.Decode(&lastOpId) == nil{
		kv.lastOpId = lastOpId
	} else {
		return false
	}
	if d.Decode(&processed) == nil{
		kv.processed = processed
	} else {
		return false
	}
	return true
}

func (kv *KVServer) applyLoop(){
	for {
		msg := <-kv.applyCh
		if msg.CommandValid{
			res1, res2 := msg.Command.(Op)
			isLeader := !res2
			var op *Op
			if !isLeader { op = &res1 } else { op = msg.Command.(*Op) }
			kv.mu.Lock()
			var ret Result
			res, ok := kv.processed[op.Id]
			if ok{
				if res.Prepared {
					ret.Value = res.Value
					ret.Err = res.Err
				}
				kv.mu.Unlock()
				if isLeader {op.DoneCh <- ret}
				continue
			}
			if lastOpId, ok := kv.lastOpId[op.ClerkId]; ok{
				if op.Id <= lastOpId{
					ret.Err = None
					kv.mu.Unlock()
					if isLeader {op.DoneCh <- ret}
					continue
				}
			}
			switch op.Type {
				case Get_op:{
					_, ok := kv.data[op.Key]
					if !ok {
						ret.Err = ErrNoKey
					} else {
						ret.Value = kv.data[op.Key]
						ret.Err = OK
					}
				}
				case Put_op:{
					kv.data[op.Key] = op.Value
					ret.Err = OK
				}
				case Append_op:{
					_, ok := kv.data[op.Key]
					if !ok {
						kv.data[op.Key] = op.Value
						ret.Err = ErrNoKey
					} else {
						kv.data[op.Key] += op.Value
						ret.Err = OK
					}
				}
			}
			//if isLeader {
			//	fmt.Println("who:", kv.me, "type:", op.Type, "key:", op.Key, "data:", kv.data[op.Key], "OpId:", op.Id, "Err:", ret.Err)
			//}
			kv.processed[op.Id] = Result{true, ret.Value, ret.Err}
			if lastOpId, ok := kv.lastOpId[op.ClerkId]; ok{
				delete(kv.processed, lastOpId)
			}
			kv.lastOpId[op.ClerkId] = op.Id
			if kv.maxraftstate > 0 && kv.maxraftstate - kv.rf.GetPersister().RaftStateSize() < 10{
				kv.snapshot(msg.CommandIndex)
			}
			kv.mu.Unlock()
			if isLeader {op.DoneCh <- ret}
		} else {
			kv.readSnapshot(msg.Snapshot)
		}
	}
}

func (kv *KVServer) Get(args *GetArgs, reply *GetReply) {
	// Your code here.
	kv.mu.Lock()
	res, ok := kv.processed[args.OpId]
	if ok{
		if res.Prepared {
			reply.WrongLeader = false
			reply.Value = res.Value
			reply.Err = res.Err
		}
		kv.mu.Unlock()
		return
	}
	if lastOpId, ok := kv.lastOpId[args.ClerkId]; ok{
		if args.OpId <= lastOpId{
			reply.Err = None
			kv.mu.Unlock()
			return
		}
	}
	kv.mu.Unlock()
	op := Op{Get_op, args.ClerkId, args.OpId, args.Key, "", "", make(chan Result)}
	_, _, isLeader := kv.rf.Start(&op)
	if !isLeader{
		reply.WrongLeader = true
	} else {
		ret := <-op.DoneCh
		reply.WrongLeader = false
		reply.Value = ret.Value
		reply.Err = ret.Err
	}
}

func (kv *KVServer) PutAppend(args *PutAppendArgs, reply *PutAppendReply) {
	// Your code here.
	kv.mu.Lock()
	res, ok := kv.processed[args.OpId]
	if ok{
		if res.Prepared {
			reply.WrongLeader = false
			reply.Err = res.Err
		}
		kv.mu.Unlock()
		return
	}
	if lastOpId, ok := kv.lastOpId[args.ClerkId]; ok{
		if args.OpId <= lastOpId{
			reply.Err = None
			kv.mu.Unlock()
			return
		}
	}
	kv.mu.Unlock()
	op := Op{0, args.ClerkId, args.OpId, args.Key, args.Value, "", make(chan Result)}
	if args.Op == "Put" {
		op.Type = Put_op
	} else {
		op.Type = Append_op
	}
	_, _, isLeader := kv.rf.Start(&op)
	if !isLeader{
		reply.WrongLeader = true
	} else {
		ret := <-op.DoneCh
		reply.WrongLeader = false
		reply.Err = ret.Err
	}
}

//
// the tester calls Kill() when a KVServer instance won't
// be needed again. you are not required to do anything
// in Kill(), but it might be convenient to (for example)
// turn off debug output from this instance.
//
func (kv *KVServer) Kill() {
	kv.rf.Kill()
	// Your code here, if desired.
}

//
// servers[] contains the ports of the set of
// servers that will cooperate via Raft to
// form the fault-tolerant key/value service.
// me is the index of the current server in servers[].
// the k/v server should store snapshots through the underlying Raft
// implementation, which should call persister.SaveStateAndSnapshot() to
// atomically save the Raft state along with the snapshot.
// the k/v server should snapshot when Raft's saved state exceeds maxraftstate bytes,
// in order to allow Raft to garbage-collect its log. if maxraftstate is -1,
// you don't need to snapshot.
// StartKVServer() must return quickly, so it should start goroutines
// for any long-running work.
//
func StartKVServer(servers []*labrpc.ClientEnd, me int, persister *raft.Persister, maxraftstate int) *KVServer {
	// call labgob.Register on structures you want
	// Go's RPC library to marshall/unmarshall.
	labgob.Register(Op{})

	kv := new(KVServer)
	kv.me = me
	kv.maxraftstate = maxraftstate

	// You may need initialization code here.

	kv.applyCh = make(chan raft.ApplyMsg)
	kv.rf = raft.Make(servers, me, persister, kv.applyCh)

	// You may need initialization code here.

	ok := kv.readSnapshot(kv.rf.GetPersister().ReadSnapshot())
	if !ok {
		kv.data = make(map[string]string)
		kv.lastOpId = make(map[int]int)
		kv.processed = make(map[int]Result)
	}
	go kv.applyLoop()

	return kv
}
