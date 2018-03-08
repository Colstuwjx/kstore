package cache

import (
	"k8s.io/apimachinery/pkg/util/sets"
)

// Index maps the indexed value to a set of keys in the cache that match on that value
type Index map[string]sets.String

// IndexFunc knows how to provide an indexed value for an object.
type IndexFunc func(obj interface{}) ([]string, error)

// Indexers maps a name to a IndexFunc
type Indexers map[string]IndexFunc

// Indices maps a name to an Index
type Indices map[string]Index

// Indexer implement index method for cache, matched values are `ObjectName`
// in the other word, Primary key for the object in cache.
type Indexer interface {
	// Retrieve list of objects that match on the named indexing function
	Index(indexName string, obj interface{}) ([]interface{}, error)

	// ListIndexFuncValues returns the list of generated values of an Index func
	ListIndexFuncValues(indexName string) []string

	// ByIndex lists object that match on the named indexing function with the exact key
	ByIndex(indexName, indexKey string) ([]interface{}, error)

	// ByMultipleIndex lists object that match on named list indexing function with the exact key
	ByMultipleIndex(indexKV map[string][]string) ([]interface{}, error)

	// GetIndexer return the indexers
	GetIndexers() Indexers

	// AddIndexers adds more indexers to this store.  If you call this after you already have data
	// in the store, the results are undefined.
	AddIndexers(newIndexers Indexers) error
}

// TODO: implement build index by object field and update, query by index, time series index is a special case.
// 时间序列的索引可以是一个有序的list，并且有固定的offset位置记录，这样一来提供固定的时间range的查询，便可以根据offset类型做高效的查询检索
