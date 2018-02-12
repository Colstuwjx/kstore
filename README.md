# kstore

kstore is a simple collector which do list watch with k8s objects and maintain local cache.

## run

```
$ go build
$ ./kstore -cluster "127.0.0.1:8080" -config "./kubeconfig" -v 3 -alsologtostderr=true
```
