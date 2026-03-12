package v1alpha1

import (
	libcnd "github.com/ifo-operator/inflightoperations/lib/condition"
)

// Labels
const (
	LabelOwnerUID         = "ifo-operator.org/owner-uid"
	LabelOwnerName        = "ifo-operator.org/owner-name"
	LabelOwnerKind        = "ifo-operator.org/owner-kind"
	LabelOwnerAPIVersion  = "ifo-operator.org/owner-apiversion"
	LabelSubjectUID       = "ifo-operator.org/subject-uid"
	LabelSubjectName      = "ifo-operator.org/subject-name"
	LabelSubjectNamespace = "ifo-operator.org/subject-namespace"
	LabelSubjectKind      = "ifo-operator.org/subject-kind"
	LabelOperation        = "ifo-operator.org/operation"
	LabelComponent        = "ifo-operator.org/component"
	LabelRuleSet          = "ifo-operator.org/ruleset"
)

// Finalizers
const (
	OperationRuleSetFinalizer = "ifo-operator.org/finalizer"
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
