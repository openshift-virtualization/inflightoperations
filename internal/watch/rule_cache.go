/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package watch

import (
	"sync"

	api "github.com/ifo-operator/inflightoperations/api/v1alpha1"
	"github.com/ifo-operator/inflightoperations/internal/metrics"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// RuleCache is a thread-safe cache for RuleSets indexed by GVR
type RuleCache struct {
	mu sync.RWMutex
	// cache maps GVR string to list of RuleSets targeting that GVR
	cache map[schema.GroupVersionResource][]api.OperationRuleSet
}

// NewRuleCache creates a new RuleCache
func NewRuleCache() *RuleCache {
	return &RuleCache{
		cache: make(map[schema.GroupVersionResource][]api.OperationRuleSet),
	}
}

// AddOrUpdateRule adds or updates an OperationRuleSet in the cache
func (r *RuleCache) AddOrUpdateRule(or *api.OperationRuleSet) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.removeRule(or)
	r.addRule(or)
}

// AddRule adds an OperationRuleSet to the cache.
func (r *RuleCache) AddRule(or *api.OperationRuleSet) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.addRule(or)
}

// RemoveRule removes an OperationRuleSet from the cache.
func (r *RuleCache) RemoveRule(or *api.OperationRuleSet) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.removeRule(or)
}

// unsafe helper; must be called with the cache locked.
func (r *RuleCache) addRule(or *api.OperationRuleSet) {
	r.cache[or.GVR()] = append(r.cache[or.GVR()], *or)
	metrics.ActiveRulesets.Inc()
}

// unsafe helper; must be called with the cache locked.
func (r *RuleCache) removeRule(or *api.OperationRuleSet) {
	key := or.GVR()
	rulesets := r.cache[key]
	for i := range rulesets {
		if rulesets[i].Name == or.Name {
			rulesets[i] = rulesets[len(rulesets)-1]
			rulesets = rulesets[:len(rulesets)-1]
			if len(rulesets) == 0 {
				delete(r.cache, key)
			} else {
				r.cache[key] = rulesets
			}
			metrics.ActiveRulesets.Dec()
			return
		}
	}
}

// List returns all rulesets targeting the specified GVR
func (r *RuleCache) List(gvr schema.GroupVersionResource) []api.OperationRuleSet {
	r.mu.RLock()
	defer r.mu.RUnlock()
	cached := r.cache[gvr]
	rulesets := make([]api.OperationRuleSet, len(cached))
	copy(rulesets, cached)
	return rulesets
}

// GVRs returns a list of all GVRs that have at least one rule
func (r *RuleCache) GVRs() []schema.GroupVersionResource {
	r.mu.RLock()
	defer r.mu.RUnlock()

	gvrs := make([]schema.GroupVersionResource, 0, len(r.cache))
	for gvr, rules := range r.cache {
		if len(rules) > 0 {
			gvrs = append(gvrs, gvr)
		}
	}
	return gvrs
}
