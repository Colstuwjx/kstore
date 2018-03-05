package store

type command struct {
	Op         string      `json:"op,omitempty"`
	ObjectName string      `json:"object_name,omitempty"`
	ObjectType string      `json:"object_type,omitempty"`
	Old        interface{} `json:"old,omitempty"`
	New        interface{} `json:"new,omitempty"`
}
