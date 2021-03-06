# kstore

kstore is a simple collector which do list watch with k8s objects and maintain local cache, we can use kstore to track schedule history, and do some statistics, monitoring job on it.

Tips: kstore is still in early and heavily development version, any break or uncompatible changes would be happened.

## Design

kstore implemented local cache to maintain k8s object data(currently ONLY pod), and build index for them.
we could filter pods by deleted/non-deleted, or POD ip, node name, even time range in the future.

## Usage

we need to build the kstore bundle, and ensure at least 3 nodes running at one cluster.

```
$ go build

# single node cluster
$ ./kstore -single -id node-0 -rdir "/tmp/raftdb/node-0" -raddr ":12000" -haddr ":11888" -cluster "127.0.0.1:8080" -config "./kubeconfig" -v 5 -alsologtostderr=true

# multiple node cluster
# node-0
# BUG: leader election still has some issues, node-0 can NOT elect itself as leader unless we set `-single`.
$ ./kstore -single -id node-0 -rdir "/tmp/raftdb/node-0" -raddr ":12000" -haddr ":11888" -cluster "127.0.0.1:8080" -config "./kubeconfig" -v 5 -alsologtostderr=true

# node-1
# join addr should be leader's http address
$ ./kstore -id node-1 -join "127.0.0.1:11888" -rdir "/tmp/raftdb/node-1" -raddr ":12001" -haddr ":11889" -cluster "127.0.0.1:8080" -config "./kubeconfig" -v 5 -alsologtostderr=true

# node-2
$ ./kstore -id node-2 -join "127.0.0.1:11888" -rdir "/tmp/raftdb/node-2" -raddr ":12002" -haddr ":11890" -cluster "127.0.0.1:8080" -config "./kubeconfig" -v 5 -alsologtostderr=true

# with specified index
$ ./kstore -single -id node-0 -rdir "/tmp/raftdb/node-0" -raddr ":12000" -haddr ":11888" -cluster "127.0.0.1:8080" -config "./kubeconfig" -v 5 -alsologtostderr=true -indexes deployName,metadata.labels,DEPLOYMENT_ID

# with sync timeout configured
$ ./kstore -single -id node-0 -rdir "/tmp/raftdb/node-0" -raddr ":12000" -haddr ":11888" -cluster "127.0.0.1:8080" -config "./kubeconfig" -v 5 -alsologtostderr=true -sync.timeout 180
```
