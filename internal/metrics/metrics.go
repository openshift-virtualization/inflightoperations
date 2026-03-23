package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
)

var factory = promauto.With(ctrlmetrics.Registry)

// Operational visibility
var (
	InFlightOperationsCreated = factory.NewCounterVec(prometheus.CounterOpts{
		Name: "ifo_inflight_operations_created_total",
		Help: "Total number of InFlightOperation resources created.",
	}, []string{"kind", "operation"})

	InFlightOperationsCompleted = factory.NewCounterVec(prometheus.CounterOpts{
		Name: "ifo_inflight_operations_completed_total",
		Help: "Total number of InFlightOperation resources completed.",
	}, []string{"kind", "operation"})

	InFlightOperationDuration = factory.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "ifo_inflight_operation_duration_seconds",
		Help:    "Duration of in-flight operations from creation to completion.",
		Buckets: prometheus.ExponentialBuckets(1, 2, 15), // 1s to ~4.5h
	}, []string{"kind", "operation"})
)

// Operator health
var (
	RulesetEvaluationErrors = factory.NewCounterVec(prometheus.CounterOpts{
		Name: "ifo_ruleset_evaluation_errors_total",
		Help: "Total number of errors evaluating OperationRuleSet CEL expressions.",
	}, []string{"ruleset"})

	ActiveWatches = factory.NewGauge(prometheus.GaugeOpts{
		Name: "ifo_active_watches",
		Help: "Number of GVRs currently being watched.",
	})

	ActiveRulesets = factory.NewGauge(prometheus.GaugeOpts{
		Name: "ifo_active_rulesets",
		Help: "Number of OperationRuleSets currently loaded.",
	})
)

// Performance
var (
	RulesetEvaluationDuration = factory.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "ifo_ruleset_evaluation_duration_seconds",
		Help:    "Duration of CEL ruleset evaluations.",
		Buckets: prometheus.DefBuckets,
	}, []string{"ruleset"})

	ProgramCacheSize = factory.NewGauge(prometheus.GaugeOpts{
		Name: "ifo_program_cache_size",
		Help: "Number of compiled CEL programs in the cache.",
	})
)
