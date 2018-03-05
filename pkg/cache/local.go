package cache

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/golang/glog"
)

type ObjectBody interface{}
type ObjectDef struct {
	Body            ObjectBody
	Type            string
	CreatedUnixTime int64
	UpdatedUnixTime int64
}

type LocalCache struct {
	// TODO: implement indexing
	ExpireTtl   time.Duration
	onlineData  map[string]*ObjectDef
	historyData map[string]*ObjectDef

	localLock sync.RWMutex
}

const (
	scavengerCheckPeriod = 30 * time.Second

	Online  = "online"
	History = "history"
)

func NewLocalCache(expireTtl time.Duration) *LocalCache {
	return &LocalCache{
		onlineData:  map[string]*ObjectDef{},
		historyData: map[string]*ObjectDef{},
		ExpireTtl:   expireTtl,
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
			for objName, obj := range lc.historyData {
				if now-obj.UpdatedUnixTime > expires {
					delete(lc.historyData, objName) // NOQA
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

func (lc *LocalCache) Data() map[string]map[string]*ObjectDef {
	lc.localLock.RLock()
	defer lc.localLock.RUnlock()

	data := make(map[string]map[string]*ObjectDef)
	data[Online] = lc.onlineData
	data[History] = lc.historyData
	return data
}

func (lc *LocalCache) Serialize() (string, error) {
	prettyJSON, err := json.MarshalIndent(lc.Data(), "", "\t")
	if err != nil {
		return "", err
	}
	return string(prettyJSON), nil
}

func (lc *LocalCache) Reset(data *map[string]map[string]*ObjectDef) error {
	lc.localLock.Lock()
	defer lc.localLock.Unlock()

	if _, exists := (*data)[Online]; !exists {
		return fmt.Errorf("%s not exists in source data", Online)
	}

	if _, exists := (*data)[History]; !exists {
		return fmt.Errorf("%s not exists in source data", History)
	}

	lc.onlineData = (*data)[Online]
	lc.historyData = (*data)[History]
	return nil
}

func (lc *LocalCache) Add(objectName, objectType string, objectBody interface{}) error {
	lc.localLock.Lock()
	defer lc.localLock.Unlock()

	if _, alreadyExists := lc.onlineData[objectName]; alreadyExists {
		return fmt.Errorf("object %s already exists in local cache", objectName)
	}

	object := &ObjectDef{
		Body:            objectBody,
		Type:            objectType,
		CreatedUnixTime: time.Now().Unix(),
		UpdatedUnixTime: time.Now().Unix(),
	}
	lc.onlineData[objectName] = object

	glog.V(5).Infof("Object [%s] %s created...", objectType, objectName)
	return nil
}

func (lc *LocalCache) Update(objectName, objectType string, oldObjectBody, newObjectBody interface{}) error {
	lc.localLock.Lock()
	defer lc.localLock.Unlock()

	// NOTE: throw warning and do update, make data keep up with the k8s real state.
	if _, exists := lc.onlineData[objectName]; !exists {
		glog.Warningf("object %s not exists in local cache", objectName)

		object := &ObjectDef{
			Body:            newObjectBody,
			Type:            objectType,
			CreatedUnixTime: time.Now().Unix(), // FIXME: using object timestamp.
			UpdatedUnixTime: time.Now().Unix(),
		}
		lc.onlineData[objectName] = object
	} else if !reflect.DeepEqual(lc.onlineData[objectName].Body, oldObjectBody) {
		glog.Warningf("object %s data not match with local cache", objectName)

		lc.onlineData[objectName].Body = newObjectBody
		lc.onlineData[objectName].UpdatedUnixTime = time.Now().Unix()
	} else {
		lc.onlineData[objectName].Body = newObjectBody
		lc.onlineData[objectName].UpdatedUnixTime = time.Now().Unix()

		glog.V(5).Infof("Object [%s] %s updated...", objectType, objectName)
	}

	return nil
}

func (lc *LocalCache) Delete(objectName, objectType string, objectBody interface{}) error {
	lc.localLock.Lock()
	defer lc.localLock.Unlock()

	if _, exists := lc.onlineData[objectName]; !exists {
		return fmt.Errorf("object %s not exists in local cache", objectName)
	}

	deletedObjectCreateTime := lc.onlineData[objectName].CreatedUnixTime
	delete(lc.onlineData, objectName) // NOQA

	if _, exists := lc.historyData[objectName]; exists {
		return fmt.Errorf("object %s exists in history local cache", objectName)
	}

	object := &ObjectDef{
		Body:            objectBody,
		Type:            objectType,
		CreatedUnixTime: deletedObjectCreateTime,
		UpdatedUnixTime: time.Now().Unix(),
	}
	lc.historyData[objectName] = object

	glog.V(5).Infof("Object [%s] %s deleted...", objectType, objectName)
	return nil
}
