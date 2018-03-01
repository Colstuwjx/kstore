package kstore

import (
	"flag"
	"time"

	"github.com/Colstuwjx/kstore/pkg/store"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

func Execute() {
	apiTarget := flag.String("cluster", "127.0.0.1:8080", "cluster target, like: 127.0.0.1:8080")
	configPath := flag.String("config", "/etc/kubernetes/kubeconfig", "kube config file path")
	flag.Parse()

	config, err := clientcmd.BuildConfigFromFlags(*apiTarget, *configPath)
	if err != nil {
		panic(err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err)
	}

	ks := store.New(clientset)
	ks.Start()

	for {
		time.Sleep(30 * time.Second)

		// test: try to stop ks listwatch.
		ks.Stop()
	}
}
