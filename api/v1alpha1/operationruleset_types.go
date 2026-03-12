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

package v1alpha1

import (
	libcnd "github.com/ifo-operator/inflightoperations/lib/condition"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// GroupVersionKind identifies a Kubernetes resource type
type GroupVersionKind struct {
	// Group is the API group of the resource
	// Empty string for core resources
	// +kubebuilder:validation:Required
	Group string `json:"group"`

	// Version is the API version of the resource
	// +kubebuilder:validation:Required
	Version string `json:"version"`

	// Kind is the kind of the resource
	// +kubebuilder:validation:Required
	Kind string `json:"kind"`
}

// Rule represents a single CEL evaluation rule
type Rule struct {
	// Operation is the name of the operation (e.g., "Migrating", "RestartRequired")
	// +kubebuilder:validation:Required
	Operation string `json:"operation"`

	// Expression is the CEL expression to evaluate against the resource
	// +kubebuilder:validation:Required
	Expression string `json:"expression"`
}

// OperationRuleSetSpec defines the desired state of OperationRuleSet
type OperationRuleSetSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	// The following markers will use OpenAPI v3 schema to validate the value
	// More info: https://book.kubebuilder.io/reference/markers/crd-validation.html

	// Component is the name of the component this rule belongs to, e.g. KubeVirt or Forklift.
	// +optional
	Component string `json:"component,omitempty"`

	// Target specifies which Kubernetes resource type to watch
	// +kubebuilder:validation:Required
	Target GroupVersionKind `json:"target"`

	// Rules contains the CEL expressions to evaluate
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:Required
	Rules []Rule `json:"rules"`

	// Namespaces determines which namespaces to watch
	// If empty, watches all namespaces
	// +optional
	Namespaces []string `json:"namespaces,omitempty"`

	// Labels to attach to all IFOs created by this ruleset.
	// +optional
	Labels map[string]string `json:"labels,omitempty"`
	// LabelExpressions are evaluated to dynamically assign labels to IFOs.
	LabelExpressions []string `json:"labelExpressions,omitempty"`
}

// OperationRuleSetStatus defines the observed state of OperationRuleSet.
type OperationRuleSetStatus struct {
	libcnd.Conditions `json:",inline"`

	// WatchActive indicates whether the dynamic watch is currently active
	WatchActive bool `json:"watchActive"`

	// LastEvaluationTime is the timestamp of the last rule evaluation
	// +optional
	LastEvaluationTime *metav1.Time `json:"lastEvaluationTime,omitempty"`

	// ObservedGeneration reflects the generation most recently observed by the controller
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,shortName=ors

// OperationRuleSet is the Schema for the OperationRuleSets API
type OperationRuleSet struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of OperationRuleSet
	// +required
	Spec OperationRuleSetSpec `json:"spec"`

	// status defines the observed state of OperationRuleSet
	// +optional
	Status OperationRuleSetStatus `json:"status,omitzero"`
}

func (r *OperationRuleSet) Rules() []Rule {
	return r.Spec.Rules
}

func (r *OperationRuleSet) GVK() schema.GroupVersionKind {
	return schema.GroupVersionKind{
		Group:   r.Spec.Target.Group,
		Version: r.Spec.Target.Version,
		Kind:    r.Spec.Target.Kind,
	}
}

func (r *OperationRuleSet) AppliesTo(subject *Subject) bool {
	for _, ns := range r.Spec.Namespaces {
		if ns == subject.GetNamespace() {
			return true
		}
	}
	return false
}

func (r *OperationRuleSet) Key() string {
	return r.GVK().String()
}

// +kubebuilder:object:root=true

// OperationRuleSetList contains a list of OperationRuleSet
type OperationRuleSetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []OperationRuleSet `json:"items"`
}

func init() {
	SchemeBuilder.Register(&OperationRuleSet{}, &OperationRuleSetList{})
}
