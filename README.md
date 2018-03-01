# kstore

kstore is a simple collector which do list watch with k8s objects and maintain local cache.

## run

```
$ go build

# single node cluster
$ ./kstore -single -cluster "127.0.0.1:8080" -config "./kubeconfig" -v 3 -alsologtostderr=true

# multiple node cluster
# node-0
# BUG: leader election still has some issues, node-0 can NOT elect itself as leader unless we set `-single`.
$ ./kstore -single -id node-0 -rdir "/tmp/raftdb/node-0" -raddr ":12000" -haddr ":11888" -cluster "172.18.33.95:8080" -config "./config" -v 5 -alsologtostderr=true

# node-1
# join addr should be leader's http address
$ ./kstore -id node-1 -join "127.0.0.1:11888" -rdir "/tmp/raftdb/node-1" -raddr ":12001" -haddr ":11889" -cluster "172.18.33.95:8080" -config "./config" -v 5 -alsologtostderr=true

# node-2
$ ./kstore -id node-2 -join "127.0.0.1:11888" -rdir "/tmp/raftdb/node-2" -raddr ":12002" -haddr ":11890" -cluster "172.18.33.95:8080" -config "./config" -v 5 -alsologtostderr=true
```
