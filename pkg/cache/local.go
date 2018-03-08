package cache

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/golang/glog"
	"k8s.io/apimachinery/pkg/util/sets"
)

type ObjectBody interface{}
type ObjectDef struct {
	Body            ObjectBody
	Type            string
	CreatedUnixTime int64
	UpdatedUnixTime int64
	Deleted         bool
}

type LocalCache struct {
	ExpireTtl    time.Duration
	OutOfDateTtl time.Duration
	data         map[string]*ObjectDef

	localLock sync.RWMutex

	// indexers maps a name to an IndexFunc
	indexers Indexers

	// indices maps a name to an Index
	indices Indices
}

const (
	scavengerCheckPeriod = 30 * time.Second
)

func NewLocalCache(indexers Indexers, indices Indices, outOfDateTtl time.Duration, expireTtl time.Duration) *LocalCache {
	return &LocalCache{
		OutOfDateTtl: outOfDateTtl,
		ExpireTtl:    expireTtl,
		data:         make(map[string]*ObjectDef),
		localLock:    sync.RWMutex{},
		indexers:     indexers,
		indices:      indices,
	}
}

func (lc *LocalCache) scavenger() {
	t := time.NewTimer(scavengerCheckPeriod)
	for {
		// TODO: add graceful stop chan.
		select {
		case <-t.C:
			t.Reset(scavengerCheckPeriod)

			// foreach each items and expires out-of-date item.
			now := time.Now().Unix()
			expires := int64(lc.ExpireTtl / time.Second)
			outOfDate := int64(lc.OutOfDateTtl / time.Second)

			// TODO: change to foreach delete indexed data.
			lc.localLock.Lock()
			glog.V(5).Info("started scheduled scavenger work...")

			for objName, obj := range lc.data {
				// FIXME: recovered data would set deleted data back
				// we need to force persistent while did scavenger work.
				if !obj.Deleted && now-obj.UpdatedUnixTime > outOfDate {
					// judge whether the data is out-of-date, and set deleted
					lc.Delete(objName, obj.Type, obj.Body, true)
					glog.V(3).Infof("set item deleted: %s", objName)
				} else if obj.Deleted && now-obj.UpdatedUnixTime > expires {
					delete(lc.data, objName) // NOQA
					lc.deleteFromIndices(obj, objName)
					glog.V(3).Infof("deleted expired item: %s", objName)
				}
			}

			lc.localLock.Unlock()
		}
	}
}

func (lc *LocalCache) Init() error {
	go lc.scavenger()
	return nil
}

func (lc *LocalCache) Data() map[string]*ObjectDef {
	lc.localLock.RLock()
	defer lc.localLock.RUnlock()

	return lc.data
}

func (lc *LocalCache) Serialize(object interface{}) (string, error) {
	prettyJSON, err := json.MarshalIndent(object, "", "\t")
	if err != nil {
		return "", err
	}
	return string(prettyJSON), nil
}

func (lc *LocalCache) SerializedData() (string, error) {
	return lc.Serialize(lc.Data())
}

func (lc *LocalCache) Reset(data *map[string]*ObjectDef) error {
	lc.localLock.Lock()
	defer lc.localLock.Unlock()

	lc.data = (*data)

	// rebuild any index
	lc.indices = Indices{}
	for key, item := range lc.data {
		lc.updateIndices(nil, item, key)
	}
	return nil
}

func (lc *LocalCache) Add(objectName, objectType string, objectBody interface{}, lockFree bool) error {
	if !lockFree {
		lc.localLock.Lock()
		defer lc.localLock.Unlock()
	}

	oldObject, alreadyExists := lc.data[objectName]
	if alreadyExists {
		glog.V(4).Infof("object %s already exists in local cache", objectName)
	}

	object := &ObjectDef{
		Body:            objectBody,
		Type:            objectType,
		CreatedUnixTime: time.Now().Unix(),
		UpdatedUnixTime: time.Now().Unix(),
	}
	lc.data[objectName] = object
	glog.V(5).Infof("Object [%s] %s created...", objectType, objectName)

	// nil != *cache.Objectdef<=nil>
	if alreadyExists {
		err := lc.updateIndices(oldObject, object, objectName)
		if err != nil {
			glog.V(4).Infof("Object [%s] %s update index failed, err: %s", objectType, objectName, err)
		}
	} else {
		err := lc.updateIndices(nil, object, objectName)
		if err != nil {
			glog.V(4).Infof("Object [%s] %s add index failed, err: %s", objectType, objectName, err)
		}
	}

	return nil
}

func (lc *LocalCache) Update(objectName, objectType string, oldObjectBody, newObjectBody interface{}, lockFree bool) error {
	if !lockFree {
		lc.localLock.Lock()
		defer lc.localLock.Unlock()
	}

	// NOTE: throw warning and do update, make data keep up with the k8s real state.
	oldObject, exists := lc.data[objectName]
	if !exists {
		glog.V(4).Infof("object %s not exists in local cache", objectName)

		object := &ObjectDef{
			Body:            newObjectBody,
			Type:            objectType,
			Deleted:         false,
			CreatedUnixTime: time.Now().Unix(), // FIXME: using object timestamp.
			UpdatedUnixTime: time.Now().Unix(),
		}
		lc.data[objectName] = object
	} else if !reflect.DeepEqual(oldObject.Body, oldObjectBody) {
		glog.V(4).Infof("object %s data not match with local cache", objectName)

		lc.data[objectName].Body = newObjectBody
		lc.data[objectName].UpdatedUnixTime = time.Now().Unix()
	} else {
		lc.data[objectName].Body = newObjectBody
		lc.data[objectName].UpdatedUnixTime = time.Now().Unix()
	}

	glog.V(5).Infof("Object [%s] %s updated...", objectType, objectName)
	err := lc.updateIndices(oldObject, lc.data[objectName], objectName)
	if err != nil {
		glog.V(4).Infof("Object [%s] %s update index failed, err: %s", objectType, objectName, err)
	}
	return nil
}

func (lc *LocalCache) Delete(objectName, objectType string, objectBody interface{}, lockFree bool) error {
	if !lockFree {
		lc.localLock.Lock()
		defer lc.localLock.Unlock()
	}

	var deletedObjectCreateTime int64

	oldObject, exists := lc.data[objectName]
	if !exists {
		glog.V(4).Infof("object %s not exists in local cache", objectName)
		deletedObjectCreateTime = time.Now().Unix() // faked time
	} else {
		if lc.data[objectName].Deleted {
			glog.V(4).Infof("object %s already deleted", objectName)
			return nil
		}

		deletedObjectCreateTime = lc.data[objectName].CreatedUnixTime
	}

	object := &ObjectDef{
		Body:            objectBody,
		Type:            objectType,
		Deleted:         true,
		CreatedUnixTime: deletedObjectCreateTime,
		UpdatedUnixTime: time.Now().Unix(),
	}
	lc.data[objectName] = object

	glog.V(5).Infof("Object [%s] %s set deleted...", objectType, objectName)

	if exists {
		err := lc.updateIndices(oldObject, object, objectName)
		if err != nil {
			glog.V(4).Infof("Object [%s] %s update index failed, err: %s", objectType, objectName, err)
		}
	} else {
		err := lc.updateIndices(nil, object, objectName)
		if err != nil {
			glog.V(4).Infof("Object [%s] %s update index failed, err: %s", objectType, objectName, err)
		}
	}

	return nil
}

// Index returns a list of items that match on the index function
// Index is thread-safe so long as you treat all items as immutable
func (lc *LocalCache) Index(indexName string, obj interface{}) ([]interface{}, error) {
	lc.localLock.RLock()
	defer lc.localLock.RUnlock()

	indexFunc := lc.indexers[indexName]
	if indexFunc == nil {
		return nil, fmt.Errorf("Index with name %s does not exist", indexName)
	}

	indexKeys, err := indexFunc(obj)
	if err != nil {
		return nil, err
	}
	index := lc.indices[indexName]

	// need to de-dupe the return list.  Since multiple keys are allowed, this can happen.
	returnKeySet := sets.String{}
	for _, indexKey := range indexKeys {
		set := index[indexKey]
		for _, key := range set.UnsortedList() {
			returnKeySet.Insert(key)
		}
	}

	list := make([]interface{}, 0, returnKeySet.Len())
	for absoluteKey := range returnKeySet {
		list = append(list, lc.data[absoluteKey])
	}
	return list, nil
}

// ByIndex returns a list of items that match an exact value on the index function
func (lc *LocalCache) ByIndex(indexName, indexKey string) ([]interface{}, error) {
	lc.localLock.RLock()
	defer lc.localLock.RUnlock()

	indexFunc := lc.indexers[indexName]
	if indexFunc == nil {
		return nil, fmt.Errorf("Index with name %s does not exist", indexName)
	}

	index := lc.indices[indexName]
	set := index[indexKey]
	list := make([]interface{}, 0, set.Len())
	for _, key := range set.List() {
		list = append(list, lc.data[key])
	}

	return list, nil
}

// ByMultipleIndex returns a list of items that both match the value for the index functions
func (lc *LocalCache) ByMultipleIndex(indexKV map[string][]string) ([]interface{}, error) {
	lc.localLock.RLock()
	defer lc.localLock.RUnlock()

	matchSet := sets.String{}
	for indexName, indexKeys := range indexKV {
		set := sets.String{}
		indexFunc := lc.indexers[indexName]
		if indexFunc == nil {
			continue
		}

		index := lc.indices[indexName]
		for _, indexKey := range indexKeys {
			set = set.Union(index[indexKey])
		}

		if matchSet.Len() == 0 {
			matchSet = set
		} else {
			matchSet = matchSet.Intersection(set)
		}
	}

	list := make([]interface{}, 0, matchSet.Len())
	for _, key := range matchSet.List() {
		list = append(list, lc.data[key])
	}

	return list, nil
}

func (lc *LocalCache) ListIndexFuncValues(indexName string) []string {
	lc.localLock.RLock()
	defer lc.localLock.RUnlock()

	index := lc.indices[indexName]
	names := make([]string, 0, len(index))
	for key := range index {
		names = append(names, key)
	}
	return names
}

func (lc *LocalCache) GetIndexers() Indexers {
	return lc.indexers
}

func (lc *LocalCache) AddIndexers(newIndexers Indexers) error {
	lc.localLock.Lock()
	defer lc.localLock.Unlock()

	if len(lc.data) > 0 {
		return fmt.Errorf("cannot add indexers to running index")
	}

	oldKeys := sets.StringKeySet(lc.indexers)
	newKeys := sets.StringKeySet(newIndexers)

	if oldKeys.HasAny(newKeys.List()...) {
		return fmt.Errorf("indexer conflict: %v", oldKeys.Intersection(newKeys))
	}

	for k, v := range newIndexers {
		lc.indexers[k] = v
	}
	return nil
}

// updateIndices modifies the objects location in the managed indexes, if this is an update, you must provide an oldObj
func (lc *LocalCache) updateIndices(oldObj interface{}, newObj interface{}, key string) error {
	// if we got an old object, we need to remove it before we add it again
	if oldObj != nil {
		lc.deleteFromIndices(oldObj, key)
	}

	for name, indexFunc := range lc.indexers {
		indexValues, err := indexFunc(newObj)
		if err != nil {
			return err
		}

		index := lc.indices[name]
		if index == nil {
			index = Index{}
			lc.indices[name] = index
		}

		for _, indexValue := range indexValues {
			set := index[indexValue]
			if set == nil {
				set = sets.String{}
				index[indexValue] = set
			}

			set.Insert(key)
		}
	}

	return nil
}

// deleteFromIndices removes the object from each of the managed indexes
func (lc *LocalCache) deleteFromIndices(obj interface{}, key string) error {
	if obj == nil {
		glog.Info("nil object found while deleting indice.")
		return nil
	}

	for name, indexFunc := range lc.indexers {
		indexValues, err := indexFunc(obj)
		if err != nil {
			return err
		}

		index := lc.indices[name]
		if index == nil {
			continue
		}

		for _, indexValue := range indexValues {
			set := index[indexValue]
			if set != nil {
				set.Delete(key)
			}
		}
	}

	return nil
}
