package store

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/golang/glog"
	"github.com/hashicorp/raft"
	"github.com/hashicorp/raft-boltdb"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	clientset "k8s.io/client-go/kubernetes"
	kcache "k8s.io/client-go/tools/cache"

	"github.com/Colstuwjx/kstore/pkg/cache"
)

const (
	// Resync period for the kube controller loop.
	resyncPeriod = 5 * time.Second
	timeout      = 5 * time.Second
	expireTtl    = 60 * 24 * time.Hour

	// raft configurations.
	retainSnapshotCount   = 2
	raftTimeout           = 10 * time.Second
	leaderElectionTimeout = 30 * time.Second

	ObjectTypePods = "pods"
)

type KStore struct {
	// local cache
	localCache *cache.LocalCache

	kubeClient clientset.Interface

	// stop chan with stop watching and graceful exit
	stopChan chan struct{}

	// Initial timeout for endpoints and services to be synced from APIServer
	initialSyncTimeout time.Duration

	// cached pods store
	podsStore kcache.Store

	// podsController  invokes registered callbacks when pods change.
	podsController kcache.Controller

	// raft and consensus mechanism
	RaftDir, RaftBind, JoinAddr string // TODO: ensure dir exists.
	raft                        *raft.Raft

	// http server configurations
	HttpBind string
}

func (ks *KStore) setPodsStore() {
	// Returns a cache.ListWatch that gets all changes to endpoints.
	ks.podsStore, ks.podsController = kcache.NewInformer(
		kcache.NewListWatchFromClient(
			ks.kubeClient.Core().RESTClient(),
			ObjectTypePods,
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
	// if ks instance is not leader, skip write operation.
	if ks.raft.State() != raft.Leader {
		glog.V(5).Info("skipped `addPodToCache` due to I'm not leader")
		return
	}

	addPod, ok := obj.(*v1.Pod)
	if !ok {
		glog.Error("received obj not pod type!")
		return
	}

	c := &command{
		Op:         "add",
		ObjectName: addPod.Name,
		ObjectType: ObjectTypePods,
		Old:        nil,
		New:        addPod,
	}
	b, err := json.Marshal(c)
	if err != nil {
		glog.Errorf("marshal received addPod err: %s", err)
		return
	}

	f := ks.raft.Apply(b, raftTimeout)
	err = f.Error()
	if err != nil {
		glog.Errorf("apply raft log while adding pod err: %s", err)
		return
	}
}

func (ks *KStore) updatePodToCache(oldObj, newObj interface{}) {
	// if ks instance is not leader, skip write operation.
	if ks.raft.State() != raft.Leader {
		glog.V(5).Info("skipped `updatePodToCache` due to I'm not leader")
		return
	}

	oldPod, ok := oldObj.(*v1.Pod)
	if !ok {
		glog.Error("received old obj not pod type!")
		return
	}

	newPod, ok := newObj.(*v1.Pod)
	if !ok {
		glog.Error("received new obj not pod type!")
		return
	}

	c := &command{
		Op:         "update",
		ObjectName: newPod.Name,
		ObjectType: ObjectTypePods,
		Old:        oldPod,
		New:        newPod,
	}
	b, err := json.Marshal(c)
	if err != nil {
		glog.Errorf("marshal received updatePod err: %s", err)
		return
	}

	f := ks.raft.Apply(b, raftTimeout)
	err = f.Error()
	if err != nil {
		glog.Errorf("apply raft log while updating pod err: %s", err)
		return
	}
}

func (ks *KStore) deletePodToCache(delObj interface{}) {
	// if ks instance is not leader, skip write operation.
	// FIXME: we may lost the delete event, and resync can NOT help us
	// as a solution, we need to check the POD update timestamp and mark deleted while it is not alive.
	if ks.raft.State() != raft.Leader {
		glog.V(5).Info("skipped `deletePodToCache` due to I'm not leader")
		return
	}

	deletePod, ok := delObj.(*v1.Pod)
	if !ok {
		glog.Error("received delete obj not pod type!")
		return
	}

	c := &command{
		Op:         "delete",
		ObjectName: deletePod.Name,
		ObjectType: ObjectTypePods,
		Old:        deletePod,
		New:        nil,
	}
	b, err := json.Marshal(c)
	if err != nil {
		glog.Errorf("marshal received deletePod err: %s", err)
		return
	}

	f := ks.raft.Apply(b, raftTimeout)
	err = f.Error()
	if err != nil {
		glog.Errorf("apply raft log while deleting pod err: %s", err)
		return
	}
}

func (ks *KStore) initRaft(enableSingleNode bool, localID string) error {
	// Setup Raft configuration.
	config := raft.DefaultConfig()
	config.LocalID = raft.ServerID(localID)

	// Setup Raft communication.
	addr, err := net.ResolveTCPAddr("tcp", ks.RaftBind) // 内部选举通信端口
	if err != nil {
		return err
	}

	transport, err := raft.NewTCPTransport(ks.RaftBind, addr, 3, 10*time.Second, os.Stderr)
	if err != nil {
		return err
	}

	// Create the snapshot store. This allows the Raft to truncate the log.
	snapshots, err := raft.NewFileSnapshotStore(ks.RaftDir, retainSnapshotCount, os.Stderr)
	if err != nil {
		return fmt.Errorf("file snapshot store: %s", err)
	}

	// Create the log store and stable store.
	logStore, err := raftboltdb.NewBoltStore(filepath.Join(ks.RaftDir, "raft.db"))
	if err != nil {
		return fmt.Errorf("new bolt store: %s", err)
	}

	// Instantiate the Raft systems.
	ra, err := raft.NewRaft(config, (*fsm)(ks), logStore, logStore, snapshots, transport)
	if err != nil {
		return fmt.Errorf("new raft: %s", err)
	}
	ks.raft = ra

	if enableSingleNode {
		configuration := raft.Configuration{
			Servers: []raft.Server{
				{
					ID:      config.LocalID,
					Address: transport.LocalAddr(),
				},
			},
		}
		ra.BootstrapCluster(configuration)
	}
	return nil
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

func (ks *KStore) Reset(data *map[string]map[string]*cache.ObjectDef) error {
	return ks.localCache.Reset(data)
}

func (ks *KStore) watchLeaderElection(checkPeriod time.Duration, waitTimeout time.Duration) error {
	// wait until leader elected.
	t := time.NewTimer(checkPeriod)
	timeout := time.After(waitTimeout)

	for {
		select {
		case <-timeout:
			return errors.New("election timeout")
		case <-t.C:
			t.Reset(checkPeriod)

			ksLeader := ks.raft.Leader()
			if ksLeader != "" {
				glog.Infof("ks leader elected: %s", ksLeader)
				return nil
			}
		}
	}
}

func postJoinRequest(joinAddr, raftAddr, nodeID string) error {
	b, err := json.Marshal(reqJoin{RemoteAddr: raftAddr, NodeId: nodeID})
	if err != nil {
		return err
	}

	resp, err := http.Post(fmt.Sprintf("http://%s/join", joinAddr), "application-type/json", bytes.NewReader(b))
	if err != nil {
		return err
	}

	defer resp.Body.Close()
	return nil
}

func (ks *KStore) Start(enableSingleNode bool, localID string) {
	glog.Info("kstore starting...")

	// firstly, async run http server
	// so we can call leader Join func and do join while initial raft peers.
	go ks.ServeHttp()

	glog.Info("initializing raft...")

	if err := ks.initRaft(enableSingleNode, localID); err != nil {
		panic(err)
	}

	if ks.JoinAddr != "" {
		if err := postJoinRequest(ks.JoinAddr, ks.RaftBind, localID); err != nil {
			glog.Fatalf("ks node %s join %s failed: %s", localID, ks.JoinAddr, err)
		}
	}

	if err := ks.watchLeaderElection(time.Second*5, leaderElectionTimeout); err != nil {
		panic(err)
	}

	glog.Info("Starting podsController...")
	go ks.podsController.Run(ks.stopChan)
	ks.waitForResourceSyncedOrDie()

	glog.Info("kstore started!")
}

func (ks *KStore) WaitUtilStop() {
	terminate := make(chan os.Signal, 1)
	signal.Notify(terminate, syscall.SIGINT, syscall.SIGTERM)
	<-terminate

	if !ks.Stopped() {
		// TODO: implement graceful shutdown cache and raft peer part.
		// FIXME: need to fix gap period while I'm leader and stopped listwatch!
		glog.V(0).Info("stopping listwatch and graceful shutdown.")
		ks.Stop()
	}

	glog.V(0).Info("GoodBye!")
}

func (ks *KStore) ServeHttp() {
	glog.Info("Starting http server...")

	ks.setHttpHandlers()
	glog.Fatal(http.ListenAndServe(ks.HttpBind, nil))
}

func (ks *KStore) Stop() {
	close(ks.stopChan)
	glog.Info("kstore listwatch stopped...")
}

func (ks *KStore) Stopped() bool {
	if ks.stopChan != nil {
		return false
	} else {
		return true
	}
}

// Join do raft peer join(ONLY leader can do this)
func (ks *KStore) Join(nodeID, addr string) error {
	glog.V(0).Infof("received join request for remote node %s at %s", nodeID, addr)

	f := ks.raft.AddVoter(raft.ServerID(nodeID), raft.ServerAddress(addr), 0, 0)
	if f.Error() != nil {
		return f.Error()
	}

	glog.V(0).Infof("node %s at %s joined successfully", nodeID, addr)
	return nil
}

func New(client clientset.Interface, httpBind, raftBind, raftDir, join string) *KStore {
	ks := &KStore{
		kubeClient:         client,
		initialSyncTimeout: timeout,
		stopChan:           make(chan struct{}),
		RaftBind:           raftBind,
		RaftDir:            raftDir,
		HttpBind:           httpBind,
		JoinAddr:           join,
	}

	ks.localCache = cache.NewLocalCache(expireTtl)
	ks.setPodsStore()
	return ks
}
