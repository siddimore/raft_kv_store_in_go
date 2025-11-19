package kv

type Command struct {
	Op    string // "set" or "get"
	Key   string
	Value string // Only used for "set" operations
}
