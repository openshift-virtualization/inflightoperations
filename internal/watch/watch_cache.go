package watch

import (
	"context"
	"fmt"
	"maps"
	"sync"

	"github.com/ifo-operator/inflightoperations/internal/metrics"
	"github.com/ifo-operator/inflightoperations/settings"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/cache"
)

func NewWatchCache() *WatchCache {
	return &WatchCache{
		watches: make(map[schema.GroupVersionResource]*Watch),
	}
}

// WatchCache must be locked before interacting with it.
type WatchCache struct {
	mu      sync.RWMutex
	watches map[schema.GroupVersionResource]*Watch
}

func (r *WatchCache) Lock() {
	r.mu.Lock()
}

func (r *WatchCache) Unlock() {
	r.mu.Unlock()
}

func (r *WatchCache) StartWithSync(gvr schema.GroupVersionResource, informer cache.SharedIndexInformer) (err error) {
	ctx, cancel := context.WithTimeout(context.Background(), settings.Settings.InformerSyncTimeout)
	defer cancel()

	w := NewWatch(gvr, informer)
	err = w.StartWithSync(ctx)
	if err != nil {
		return
	}
	r.watches[gvr] = w
	return
}

func (r *WatchCache) Start(gvr schema.GroupVersionResource, informer cache.SharedIndexInformer) {
	w := NewWatch(gvr, informer)
	w.Start()
	r.watches[gvr] = w
	metrics.ActiveWatches.Inc()
}

func (r *WatchCache) Stop(gvr schema.GroupVersionResource) {
	w, ok := r.watches[gvr]
	if !ok {
		return
	}
	w.Stop()
	delete(r.watches, gvr)
	metrics.ActiveWatches.Dec()
}

func (r *WatchCache) Exists(gvr schema.GroupVersionResource) (ok bool) {
	_, ok = r.watches[gvr]
	return
}

func (r *WatchCache) Prune(gvrs []schema.GroupVersionResource) {
	keep := make(map[schema.GroupVersionResource]bool, len(gvrs))
	for _, gvr := range gvrs {
		keep[gvr] = true
	}
	for gvr := range maps.Keys(r.watches) {
		if !keep[gvr] {
			r.Stop(gvr)
		}
	}
}

func NewWatch(gvr schema.GroupVersionResource, informer cache.SharedIndexInformer) (w *Watch) {
	w = &Watch{
		gvr:      gvr,
		informer: informer,
	}
	return
}

// Watch tracks the state of a single dynamic watch
type Watch struct {
	gvr      schema.GroupVersionResource
	informer cache.SharedIndexInformer
	stopCh   chan struct{}
	running  bool
}

func (r *Watch) StartWithSync(ctx context.Context) (err error) {
	r.Start()
	if !cache.WaitForCacheSync(ctx.Done(), r.informer.HasSynced) {
		r.Stop()
		err = fmt.Errorf("failed to sync cache for GVR %s", r.gvr.String())
		return
	}
	return
}

func (r *Watch) Start() {
	r.stopCh = make(chan struct{})
	r.running = true
	go r.informer.Run(r.stopCh)
}

func (r *Watch) Stop() {
	close(r.stopCh)
	r.running = false
}
