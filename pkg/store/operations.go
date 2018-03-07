package store

/*
   NOTE: operations implements all exposed operations with cache.
*/

func (ks *KStore) GetCacheAsJson() (string, error) {
	return ks.localCache.SerializedData()
}

func (ks *KStore) GetCacheByIndex(indexName, indexKey string) (string, error) {
	objects, err := ks.localCache.ByIndex(indexName, indexKey)
	if err != nil {
		return "", err
	}

	return ks.localCache.Serialize(objects)
}
