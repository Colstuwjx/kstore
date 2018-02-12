package store

import (
	"time"

	"github.com/golang/glog"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/util/wait"
	clientset "k8s.io/client-go/kubernetes"
	kcache "k8s.io/client-go/tools/cache"
)

var (
	// Resync period for the kube controller loop.
	resyncPeriod = 5 * time.Second

	timeout = 5 * time.Second
)

type KStore struct {
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
	if addPods, ok := obj.(*v1.Pod); ok {
		glog.V(3).Infof("received add pods: %v", addPods)
	}
}

func (ks *KStore) updatePodToCache(oldObj, newObj interface{}) {
	oldPods, ok := oldObj.(*v1.Pod)
	if ok {
		glog.V(3).Infof("received old pods: %v", oldPods)
	}

	newPods, ok := newObj.(*v1.Pod)
	if ok {
		glog.V(3).Infof("received new pods: %v", newPods)
	}
}

func (ks *KStore) deletePodToCache(obj interface{}) {
	if delPods, ok := obj.(*v1.Pod); ok {
		glog.V(3).Infof("deleted pods: %v", delPods)
	}
}

func (ks *KStore) Start() {
	glog.V(3).Info("Starting podsController...")
	go ks.podsController.Run(wait.NeverStop)

	ks.waitForResourceSyncedOrDie()

	// TODO: offer http api to get/set cache.
	for {
		time.Sleep(1 * time.Second)
	}
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
				glog.V(3).Info("Initialized pods from apiserver...")
				return
			}
			glog.V(3).Info("Waiting for pods to be initialized from apiserver...")
		}
	}
}

func New(client clientset.Interface) *KStore {
	ks := &KStore{
		kubeClient:         client,
		initialSyncTimeout: timeout,
	}

	ks.setPodsStore()
	return ks
}
