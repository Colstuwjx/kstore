package store

import (
	"time"

	"github.com/golang/glog"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/util/wait"
	clientset "k8s.io/client-go/kubernetes"
	kcache "k8s.io/client-go/tools/cache"

	"github.com/Colstuwjx/kstore/pkg/cache"
)

var (
	// Resync period for the kube controller loop.
	resyncPeriod = 5 * time.Second
	timeout      = 5 * time.Second
	expireTtl    = 60 * 24 * time.Hour
)

type KStore struct {
	// local cache
	localCache *cache.LocalCache

	kubeClient clientset.Interface

	// Initial timeout for endpoints and services to be synced from APIServer
	initialSyncTimeout time.Duration

	// cached pods store
	podsStore kcache.Store

	// podsController  invokes registered callbacks when pods change.
	podsController kcache.Controller
}

func (ks *KStore) setPodsStore() {
	// Returns a cache.ListWatch that gets all changes to endpoints.
	ks.podsStore, ks.podsController = kcache.NewInformer(
		kcache.NewListWatchFromClient(
			ks.kubeClient.Core().RESTClient(),
			"pods",
			v1.NamespaceAll,
			fields.Everything()),
		&v1.Pod{},
		resyncPeriod,

		kcache.ResourceEventHandlerFuncs{
			AddFunc:    ks.addPodToCache,
			UpdateFunc: ks.updatePodToCache,
			DeleteFunc: ks.deletePodToCache,
		},
	)
}

func (ks *KStore) addPodToCache(obj interface{}) {
	if addPod, ok := obj.(*v1.Pod); ok {
		err := ks.localCache.Add(addPod.Name, addPod)
		if err != nil {
			glog.Errorf("add pod err: %s", err)
			return
		}
	} else {
		glog.Error("received obj not pod type!")
	}
}

func (ks *KStore) updatePodToCache(oldObj, newObj interface{}) {
	oldPod, ok := oldObj.(*v1.Pod)
	if !ok {
		glog.Error("received oldObj not pod type!")
		return
	}

	newPod, ok := newObj.(*v1.Pod)
	if !ok {
		glog.Error("received newObj not pod type!")
		return
	}

	if oldPod.Name != newPod.Name {
		glog.Error("received oldObj, newObj with inconsistent name!")
		return
	}

	err := ks.localCache.Update(newPod.Name, oldPod, newPod)
	if err != nil {
		glog.Error("update pod err: %s", err)
	}
}

func (ks *KStore) deletePodToCache(delObj interface{}) {
	if delPod, ok := delObj.(*v1.Pod); ok {
		err := ks.localCache.Delete(delPod.Name, delPod)
		if err != nil {
			glog.Error("delete pod err: %s", err)
			return
		}
	} else {
		glog.Error("received delObj not pod type!")
	}
}

func (ks *KStore) Start() {
	glog.Info("Starting podsController...")
	go ks.podsController.Run(wait.NeverStop)

	ks.waitForResourceSyncedOrDie()

	// TODO: offer http api to get/set cache, now we just return.
}

func (ks *KStore) waitForResourceSyncedOrDie() {
	// Wait for both controllers have completed an initial resource listing
	timeout := time.After(ks.initialSyncTimeout)
	ticker := time.NewTicker(500 * time.Millisecond)

	defer ticker.Stop()
	for {
		select {
		case <-timeout:
			panic("Timeout waiting for initialization")
		case <-ticker.C:
			if ks.podsController.HasSynced() {
				glog.Info("Initialized pods from apiserver...")
				return
			}
			glog.Info("Waiting for pods to be initialized from apiserver...")
		}
	}
}

func New(client clientset.Interface) *KStore {
	ks := &KStore{
		kubeClient:         client,
		initialSyncTimeout: timeout,
	}

	ks.localCache = cache.NewLocalCache(expireTtl)
	ks.setPodsStore()
	return ks
}
