package watch

import (
	"context"
	"time"

	api "github.com/openshift-virtualization/inflightoperations/api/v1alpha1"
	"github.com/openshift-virtualization/inflightoperations/internal/evaluator"
	"github.com/openshift-virtualization/inflightoperations/internal/metrics"
	liberr "github.com/openshift-virtualization/inflightoperations/lib/error"
	"github.com/openshift-virtualization/inflightoperations/lib/logging"
	"github.com/openshift-virtualization/inflightoperations/settings"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var Settings = &settings.Settings

type Watcher struct {
	client          client.Client
	dynamicClient   dynamic.Interface
	informerFactory dynamicinformer.DynamicSharedInformerFactory
	log             logging.LevelLogger
	cache           *WatchCache
	rules           *RuleCache
	evaluator       evaluator.Evaluator
	operations      *Operations
}

func NewWatcher(cl client.Client, dynamicClient dynamic.Interface, rules *RuleCache, eval evaluator.Evaluator, log logging.LevelLogger) *Watcher {
	factory := dynamicinformer.NewFilteredDynamicSharedInformerFactory(
		dynamicClient,
		Settings.K8SInformerResync,
		metav1.NamespaceAll,
		nil,
	)
	return &Watcher{
		client:          cl,
		dynamicClient:   dynamicClient,
		informerFactory: factory,
		log:             log,
		cache:           NewWatchCache(),
		rules:           rules,
		evaluator:       eval,
		operations: &Operations{
			client: cl,
			log:    log,
		},
	}
}

// Start blocks until the context is cancelled, then shuts down all watches.
// This implements the controller-runtime Runnable interface.
func (r *Watcher) Start(ctx context.Context) error {
	<-ctx.Done()
	r.Shutdown()
	return nil
}

// Shutdown stops all active watches and the informer factory.
func (r *Watcher) Shutdown() {
	r.cache.Lock()
	defer r.cache.Unlock()
	r.cache.StopAll()
	r.informerFactory.Shutdown()
}

// Register a new watch for a GVR.
func (r *Watcher) Register(gvr schema.GroupVersionResource) (err error) {
	r.cache.Lock()
	defer func() {
		r.cache.Unlock()
		if err != nil {
			r.log.Error(err, "Failed to register watch.", "gvr", gvr)
		}
	}()
	if r.cache.Exists(gvr) {
		r.log.V(4).Info("Watch already registered.", "gvr", gvr.String())
		return
	}
	r.log.V(4).Info("Registering watch.", "gvr", gvr.String())
	informer := r.makeInformer(gvr)
	err = r.addHandlers(informer, gvr)
	if err != nil {
		return
	}
	r.cache.Start(gvr, informer)
	r.log.V(0).Info("Watch registered.", "gvr", gvr.String())
	return
}

func (r *Watcher) Prune() {
	r.cache.Lock()
	defer r.cache.Unlock()
	r.cache.Prune(r.rules.GVRs())
}

func (r *Watcher) addHandlers(informer cache.SharedIndexInformer, gvr schema.GroupVersionResource) (err error) {
	_, err = informer.AddEventHandler(&cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj any) {
			iErr := r.handle(obj, gvr)
			if iErr != nil {
				r.log.Error(iErr, "failed to handle Add event", "gvr", gvr)
			}
		},
		UpdateFunc: func(_, obj any) {
			iErr := r.handle(obj, gvr)
			if iErr != nil {
				r.log.Error(iErr, "failed to handle Update event", "gvr", gvr)
			}
		},
		DeleteFunc: func(obj any) {
			iErr := r.handleDelete(obj, gvr)
			if iErr != nil {
				r.log.Error(iErr, "failed to handle Delete event", "gvr", gvr)
			}
		},
	})
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	return
}

func (r *Watcher) makeInformer(gvr schema.GroupVersionResource) cache.SharedIndexInformer {
	return r.informerFactory.ForResource(gvr).Informer()
}

func (r *Watcher) handle(obj any, gvr schema.GroupVersionResource) (err error) {
	subject, ok := obj.(*api.Subject)
	if !ok {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), Settings.K8SAPITimeout)
	defer cancel()
	rulesets := r.rules.List(gvr)
	if len(rulesets) == 0 {
		r.log.V(4).Info("No rulesets for GVR", "gvr", gvr.String())
		return err
	}

	detected := make(map[string]bool)
	results := []evaluator.RuleSetResult{}
	for _, ruleset := range rulesets {
		if !ruleset.AppliesTo(subject) {
			continue
		}
		var result evaluator.RuleSetResult
		evalStart := time.Now()
		result, err = r.evaluator.EvaluateRuleSet(subject, &ruleset)
		metrics.RulesetEvaluationDuration.WithLabelValues(ruleset.Name).Observe(time.Since(evalStart).Seconds())
		if err != nil {
			metrics.RulesetEvaluationErrors.WithLabelValues(ruleset.Name).Inc()
			r.log.Error(err, "Failed to evaluate ruleset", "ruleset", ruleset, "subject", subject.GetName(), "namespace", subject.GetNamespace())
			continue
		}
		results = append(results, result)
		for _, operation := range result.Operations {
			detected[operation] = true
		}
	}
	list, err := r.operations.List(ctx, subject)
	if err != nil {
		r.log.Error(err, "Failed to list operations", "subject", subject.GetName(), "namespace", subject.GetNamespace())
		return err
	}
	for _, ifo := range list.Items {
		if !ifo.PastDebounceThreshold() {
			_, found := detected[ifo.Spec.Operation]
			if !found {
				r.markCompleted(ctx, subject, &ifo)
				r.cleanupCompleted(ctx, &ifo)
			}
		}
	}

	for _, result := range results {
		for _, operation := range result.Operations {
			op := r.operations.Build(subject, operation, result.RuleSet, result.Labels)
			op, err = r.operations.Ensure(ctx, op)
			if err != nil {
				r.log.Error(err, "Failed to ensure operation", "subject", subject.GetName(), "namespace", subject.GetNamespace())
				continue
			}
			op.MarkDetection(subject, []string{result.RuleSet.Name})
			err = r.client.Status().Update(ctx, op)
			if err != nil {
				err = liberr.Wrap(err)
				r.log.Error(err, "Failed to update operation status", "subject", subject.GetName(), "namespace", subject.GetNamespace())
				continue
			}
		}
	}
	return err
}

func (r *Watcher) handleDelete(obj any, _ schema.GroupVersionResource) (err error) {
	subject, ok := obj.(*api.Subject)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), Settings.K8SAPITimeout)
	defer cancel()
	err = r.operations.DeleteAll(ctx, subject)
	if err != nil {
		return
	}
	return
}

// markCompleted makes a best-effort attempt to mark an IFO completed.
func (r *Watcher) markCompleted(ctx context.Context, subject *api.Subject, ifo *api.InFlightOperation) {
	ifo.MarkCompleted(subject)
	err := r.client.Status().Update(ctx, ifo)
	if err != nil {
		err = liberr.Wrap(err)
		r.log.Error(err, "Failed to update operation status", "subject", subject.GetName(), "namespace", subject.GetNamespace(), "ifo", ifo.Name)
		return
	}
	kind := ifo.Spec.Subject.Kind
	operation := ifo.Spec.Operation
	metrics.InFlightOperationsCompleted.WithLabelValues(kind, operation).Inc()
	metrics.InFlightOperationDuration.WithLabelValues(kind, operation).Observe(time.Since(ifo.CreationTimestamp.Time).Seconds())
}

// cleanupCompleted makes a best-effort attempt to remove an IFO.
func (r *Watcher) cleanupCompleted(ctx context.Context, ifo *api.InFlightOperation) {
	if Settings.RetainCompletedIFOs {
		return
	}
	err := r.client.Delete(ctx, ifo)
	if err != nil {
		err = liberr.Wrap(err)
		r.log.Error(err, "Unable to cleanup completed IFO", "ifo", ifo.GetName())
	}
}
