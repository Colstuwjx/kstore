# ROADMAP

## step 1

- [x] Implement local cache struct;
- [x] Do list watch with k8s;
- [x] Support soft delete (so we can review the pod schedule history;
- [x] ONLY support POD watch right now;
- [x] Basic CURD operations;
- [x] Indexing;
- [ ] Simply import & export tool

## step 2

- [x] Leader election and replication, make kstore high available;
- [x] Snapshot, WAL and persistent;
- [x] Implement TTL and sync remote backend by period(currently via raft);
- [ ] Ratelimit with crashloop pods and customize controller like kubewatch project
