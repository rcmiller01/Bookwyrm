package cache

import (
	"time"

	"github.com/dgraph-io/ristretto"
)

type Cache interface {
	Get(key string) (interface{}, bool)
	Set(key string, value interface{}, ttl time.Duration)
	Delete(key string)
}

type ristrettoCache struct {
	r *ristretto.Cache
}

func NewRistrettoCache() (Cache, error) {
	r, err := ristretto.NewCache(&ristretto.Config{
		NumCounters: 1e7,
		MaxCost:     1 << 30, // 1 GB
		BufferItems: 64,
	})
	if err != nil {
		return nil, err
	}
	return &ristrettoCache{r: r}, nil
}

func (c *ristrettoCache) Get(key string) (interface{}, bool) {
	return c.r.Get(key)
}

func (c *ristrettoCache) Set(key string, value interface{}, ttl time.Duration) {
	c.r.SetWithTTL(key, value, 1, ttl)
}

func (c *ristrettoCache) Delete(key string) {
	c.r.Del(key)
}
