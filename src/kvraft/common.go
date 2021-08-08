package raftkv

const (
	OK       = "OK"
	ErrNoKey = "ErrNoKey"
	None	 = "None"
)

type Err string

// Put or Append
type PutAppendArgs struct {
	Key   string
	Value string
	Op    string // "Put" or "Append"
	// You'll have to add definitions here.
	// Field names must start with capital letters,
	// otherwise RPC will break.

	ClerkId int
	OpId	int
}

type PutAppendReply struct {
	WrongLeader bool
	Err         Err
}

type GetArgs struct {
	Key string
	// You'll have to add definitions here.

	ClerkId	int
	OpId	int
}

type GetReply struct {
	WrongLeader bool
	Err         Err
	Value       string
}
