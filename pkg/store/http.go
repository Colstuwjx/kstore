package store

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type reqJoin struct {
	RemoteAddr string `json:"remote_addr"`
	NodeId     string `json:"node_id"`
}

// setHandlers initialize http handlers
func (ks *KStore) setHttpHandlers() {
	http.HandleFunc("/pods/all", func(w http.ResponseWriter, req *http.Request) {
		serializedJSON, err := ks.GetCacheAsJson()
		if err == nil {
			fmt.Fprint(w, serializedJSON)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprint(w, err)
		}
	})

	http.HandleFunc("/join", func(w http.ResponseWriter, req *http.Request) {
		join := reqJoin{}
		if err := json.NewDecoder(req.Body).Decode(&join); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		if err := ks.Join(join.NodeId, join.RemoteAddr); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		fmt.Fprint(w, "joined!")
	})
}
