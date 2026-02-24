package v1alpha1

import (
	libcnd "github.com/mansam/inflightoperations/lib/condition"
)

// Finalizers
const (
	OperationRuleSetFinalizer = "ifo.kubevirt.io/finalizer"
)

// Reasons
const (
	ReasonCompleted         = "Completed"
	ReasonGVKNotFound       = "GVKNotFound"
	ReasonInvalidExpression = "InvalidExpression"
	ReasonWatchSetupFailed  = "WatchSetupFailed"
	ReasonWatchActive       = "WatchActive"
)

// Condition types
const (
	TypeReady         = "Ready"
	TypeInvalidTarget = "InvalidTarget"
	TypeInvalidRule   = "InvalidRule"
	TypeWatchFailed   = "WatchFailed"
	TypeValidated     = "Validated"
)

// Condition status
const (
	True  = libcnd.True
	False = libcnd.False
)

// Condition categories
const (
	CategoryRequired = libcnd.Required
	CategoryAdvisory = libcnd.Advisory
	CategoryCritical = libcnd.Critical
)
