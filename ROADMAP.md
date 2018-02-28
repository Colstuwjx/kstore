# ROADMAP

## step 1

- [x] Implement local cache struct;
- [x] Do list watch with k8s;
- [x] Support soft delete (so we can review the pod schedule history;
- [x] ONLY support POD watch right now;
- [] Basic CURD operations

## step 2

- [] Leader election and replication, make kstore high available;
- [] Implement TTL and sync remote backend by period;
- [] Ratelimit with crashloop pods and customize controller like kubewatch project
