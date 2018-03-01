package store

/*
   NOTE: operations implements all exposed operations with cache.
*/

func (ks *KStore) GetCacheAsJson() (string, error) {
	return ks.localCache.Serialize()
}
