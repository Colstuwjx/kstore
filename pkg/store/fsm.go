package store

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/Colstuwjx/kstore/pkg/cache"
	"github.com/golang/glog"
	"github.com/hashicorp/raft"
)

type fsm KStore

// Apply applies a Raft log entry to the key-value store.
func (f *fsm) Apply(l *raft.Log) interface{} {
	var c command
	if err := json.Unmarshal(l.Data, &c); err != nil {
		panic(fmt.Sprintf("failed to unmarshal command: %s", err.Error()))
	}

	// FIXME: also apply timestamps rather than set time in apply time.
	switch c.Op {
	case "add":
		return f.applyAdd(c.ObjectName, c.ObjectType, c.New)
	case "update":
		return f.applyUpdate(c.ObjectName, c.ObjectType, c.Old, c.New)
	case "delete":
		return f.applyDelete(c.ObjectName, c.ObjectType, c.Old)
	default:
		panic(fmt.Sprintf("unrecognized command op: %s", c.Op))
	}
}

// Snapshot returns a snapshot of the kstore local cache data.
func (f *fsm) Snapshot() (raft.FSMSnapshot, error) {
	data := f.localCache.Data()
	return &fsmSnapshot{store: data}, nil
}

// Restore stores the kstore to a previous state.
func (f *fsm) Restore(rc io.ReadCloser) error {
	o := make(map[string]*cache.ObjectDef)
	if err := json.NewDecoder(rc).Decode(&o); err != nil {
		return err
	}

	// Set the state from the snapshot, no lock required according to
	// Hashicorp docs.
	return f.localCache.Reset(&o)
}

func (f *fsm) applyAdd(objectName, objectType string, newObj interface{}) interface{} {
	err := f.localCache.Add(objectName, objectType, newObj, false)
	if err != nil {
		glog.Errorf("add %s %s err: %s", objectType, objectName, err)
		return nil
	}

	return nil
}

func (f *fsm) applyUpdate(objectName, objectType string, oldObj, newObj interface{}) interface{} {
	err := f.localCache.Update(objectName, objectType, oldObj, newObj, false)
	if err != nil {
		glog.Errorf("update %s %s err: %s", objectType, objectName, err)
		return nil
	}

	return nil
}

func (f *fsm) applyDelete(objectName, objectType string, oldObj interface{}) interface{} {
	err := f.localCache.Delete(objectName, objectType, oldObj, false)
	if err != nil {
		glog.Errorf("delete %s %s err: %s", objectType, objectName, err)
		return nil
	}

	return nil
}
