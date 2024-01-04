package cache

import (
	"context"

	"golang.org/x/sync/singleflight"
)

func WithSingleFlight(c Cache) Cache {
	return &singleFlightCache{Cache: c}
}

type singleFlightCache struct {
	sf singleflight.Group

	Cache
}

func (c *singleFlightCache) Get(ctx context.Context, key string) ([]byte, error) {
	ch := c.sf.DoChan(key, func() (any, error) {
		return c.Cache.Get(ctx, key)
	})

	select {
	case r := <-ch:
		return r.Val.([]byte), r.Err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (c *singleFlightCache) Delete(ctx context.Context, key string) ([]byte, error) {
	ch := c.sf.DoChan(key, func() (any, error) {
		return c.Cache.Delete(ctx, key)
	})

	select {
	case r := <-ch:
		return r.Val.([]byte), r.Err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (c *singleFlightCache) Set(ctx context.Context, key string, entry []byte) error {
	ch := c.sf.DoChan(key, func() (any, error) {
		return entry, c.Cache.Set(ctx, key, entry)
	})

	select {
	case r := <-ch:
		return r.Err
	case <-ctx.Done():
		return ctx.Err()
	}
}
