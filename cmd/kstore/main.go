package kstore

import (
	"errors"
	"flag"
	"github.com/Colstuwjx/kstore/pkg/store"
	"strings"

	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

type indexListFlags struct {
	Data sets.String // indexes definition with method,category,indexName(optional)
}

func (i *indexListFlags) String() string {
	return strings.Join(i.Data.List(), ",")
}

func (i *indexListFlags) Set(value string) error {
	strPair := strings.Split(value, ",")
	switch len(strPair) {
	case 2, 3:
		i.Data.Insert(value)
		return nil
	default:
		return errors.New("wrong comma delimeter for indexList flag")
	}
}

func Execute() {
	// k8s configs
	apiTarget := flag.String("cluster", "127.0.0.1:8080", "cluster target, like: 127.0.0.1:8080")
	configPath := flag.String("config", "/etc/kubernetes/kubeconfig", "kube config file path")

	// raft configs
	enableSingleNode := flag.Bool("single", false, "bootstrap single node cluster")
	localID := flag.String("id", "node-0", "raft peer node id")
	raftDir := flag.String("rdir", "/tmp/kstore", "raft db dir")
	raftBind := flag.String("raddr", ":12000", "raft bind addr")
	join := flag.String("join", "", "set join address, if any")
	syncTimeout := flag.Int("sync.timeout", 120, "set initial sync timeout seconds")

	// http configs
	httpBind := flag.String("haddr", ":11888", "http bind addr")

	// index configs
	indexes := indexListFlags{
		Data: sets.String{},
	}

	// initial index config
	initialIndexes := []string{
		"deleted,delete,",
		"podIP,status.podIP,",
		"nodeName,spec.nodeName,",
		"namespace,metadata.namespace,",
		"env.appId,spec.containers.env,APP_ID",
	}
	indexes.Data.Insert(initialIndexes...)

	flag.Var(&indexes, "indexes", "set index list item with indexMethod,indexCategory,indexName fmt")
	flag.Parse()

	config, err := clientcmd.BuildConfigFromFlags(*apiTarget, *configPath)
	if err != nil {
		panic(err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err)
	}

	ks := store.New(clientset, *httpBind, *raftBind, *raftDir, *join, *syncTimeout, indexes.Data)
	ks.Start(*enableSingleNode, *localID)
	ks.WaitUtilStop()
}
