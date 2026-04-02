package v1alpha1

import (
	libcnd "github.com/openshift-virtualization/inflightoperations/lib/condition"
)

// Labels
const (
	LabelOwnerUID         = "ifo.kubevirt.io/owner-uid"
	LabelOwnerName        = "ifo.kubevirt.io/owner-name"
	LabelOwnerKind        = "ifo.kubevirt.io/owner-kind"
	LabelOwnerGroup       = "ifo.kubevirt.io/owner-group"
	LabelOwnerVersion     = "ifo.kubevirt.io/owner-version"
	LabelSubjectUID       = "ifo.kubevirt.io/subject-uid"
	LabelSubjectName      = "ifo.kubevirt.io/subject-name"
	LabelSubjectNamespace = "ifo.kubevirt.io/subject-namespace"
	LabelSubjectKind      = "ifo.kubevirt.io/subject-kind"
	LabelOperation        = "ifo.kubevirt.io/operation"
	LabelComponent        = "ifo.kubevirt.io/component"
	LabelRuleSet          = "ifo.kubevirt.io/ruleset"
	LabelCorrelationGroup = "ifo.kubevirt.io/correlation-group"
	LabelCorrelationRole  = "ifo.kubevirt.io/correlation-role"
)

// Correlation role values
const (
	CorrelationRoleRoot  = "root"
	CorrelationRoleChild = "child"
)

// Finalizers
const (
	OperationRuleSetFinalizer = "ifo.kubevirt.io/finalizer"
)

// Reasons
const (
	ReasonCompleted         = "Completed"
	ReasonGVRNotFound       = "GVRNotFound"
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
