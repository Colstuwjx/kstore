package kstore

import (
	"flag"
	"github.com/Colstuwjx/kstore/pkg/store"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

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

	// http configs
	httpBind := flag.String("haddr", ":11888", "http bind addr")
	flag.Parse()

	config, err := clientcmd.BuildConfigFromFlags(*apiTarget, *configPath)
	if err != nil {
		panic(err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err)
	}

	ks := store.New(clientset, *httpBind, *raftBind, *raftDir, *join)
	ks.Start(*enableSingleNode, *localID)
	ks.WaitUtilStop()
}
