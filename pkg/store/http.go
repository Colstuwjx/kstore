package store

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

type reqJoin struct {
	RemoteAddr string `json:"remote_addr"`
	NodeId     string `json:"node_id"`
}

// setHandlers initialize http handlers
func (ks *KStore) setHttpHandlers() {
	// TODO: implement time range query API for specified field(indexed).
	http.HandleFunc("/pods/all", func(w http.ResponseWriter, req *http.Request) {
		serializedJSON, err := ks.GetCacheAsJson()
		if err == nil {
			fmt.Fprint(w, serializedJSON)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprint(w, err)
		}
	})

	http.HandleFunc("/indexes", func(w http.ResponseWriter, req *http.Request) {
		indexName := req.URL.Query().Get("index_name")
		if indexName == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		indices, err := ks.GetIndiceByName(indexName)
		if err == nil {
			fmt.Fprint(w, indices)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprint(w, err)
		}
	})

	http.HandleFunc("/pods/indexed", func(w http.ResponseWriter, req *http.Request) {
		indexValues, err := url.ParseQuery(req.URL.RawQuery)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprint(w, err)
			return
		}

		if len(indexValues) == 0 {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		serializedJSON, err := ks.GetCacheByMultipleIndex(indexValues)
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
