package cache

import (
	"fmt"
	"reflect"
	"sync"
	"time"
)

type ObjectBody interface{}
type ObjectDef struct {
	Body            ObjectBody
	CreatedUnixTime int64
	UpdatedUnixTime int64
}

type LocalCache struct {
	ExpireTtl   time.Duration
	OnlineData  map[string]*ObjectDef
	HistoryData map[string]*ObjectDef

	localLock sync.RWMutex
}

const (
	scavengerCheckPeriod = 30 * time.Second
)

func NewLocalCache(expireTtl time.Duration) *LocalCache {
	return &LocalCache{
		OnlineData:  map[string]*ObjectDef{},
		HistoryData: map[string]*ObjectDef{},
		ExpireTtl:   expireTtl, // TODO.
		localLock:   sync.RWMutex{},
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
			expires := int64(lc.ExpireTtl)

			lc.localLock.Lock()
			for objName, obj := range lc.HistoryData {
				if now-obj.UpdatedUnixTime > expires {
					delete(lc.HistoryData, objName) // NOQA
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

func (lc *LocalCache) Add(objectName string, objectBody interface{}) error {
	lc.localLock.Lock()
	defer lc.localLock.Unlock()

	if _, alreadyExists := lc.OnlineData[objectName]; alreadyExists {
		return fmt.Errorf("object %s already exists in local cache", objectName)
	}

	object := &ObjectDef{
		Body:            objectBody,
		CreatedUnixTime: time.Now().Unix(),
		UpdatedUnixTime: time.Now().Unix(),
	}
	lc.OnlineData[objectName] = object
	return nil
}

func (lc *LocalCache) Update(objectName string, oldObjectBody, newObjectBody interface{}) error {
	lc.localLock.Lock()
	defer lc.localLock.Unlock()

	if _, exists := lc.OnlineData[objectName]; !exists {
		return fmt.Errorf("object %s not exists in local cache", objectName)
	}

	if !reflect.DeepEqual(lc.OnlineData[objectName].Body, oldObjectBody) {
		return fmt.Errorf("object %s data not match with local cache", objectName)
	}

	lc.OnlineData[objectName].Body = newObjectBody
	lc.OnlineData[objectName].UpdatedUnixTime = time.Now().Unix()
	return nil
}

func (lc *LocalCache) Delete(objectName string, objectBody interface{}) error {
	lc.localLock.Lock()
	defer lc.localLock.Unlock()

	if _, exists := lc.OnlineData[objectName]; !exists {
		return fmt.Errorf("object %s not exists in local cache", objectName)
	}

	deletedObjectCreateTime := lc.OnlineData[objectName].CreatedUnixTime
	delete(lc.OnlineData, objectName) // NOQA

	if _, exists := lc.HistoryData[objectName]; exists {
		return fmt.Errorf("object %s exists in history local cache", objectName)
	}

	object := &ObjectDef{
		Body:            objectBody,
		CreatedUnixTime: deletedObjectCreateTime,
		UpdatedUnixTime: time.Now().Unix(),
	}
	lc.HistoryData[objectName] = object
	return nil
}
