/*
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

package main

// Minimal OLM CSV types. Defined here to avoid importing
// github.com/operator-framework/api and its transitive dependencies.

type ClusterServiceVersion struct {
	APIVersion string      `json:"apiVersion"`
	Kind       string      `json:"kind"`
	Metadata   CSVMetadata `json:"metadata"`
	Spec       CSVSpec     `json:"spec"`
}

type CSVMetadata struct {
	Name        string            `json:"name"`
	Namespace   string            `json:"namespace"`
	Annotations map[string]string `json:"annotations,omitempty"`
}

type CSVSpec struct {
	DisplayName               string                    `json:"displayName"`
	Description               string                    `json:"description"`
	Keywords                  []string                  `json:"keywords,omitempty"`
	Links                     []Link                    `json:"links,omitempty"`
	Maintainers               []Maintainer              `json:"maintainers,omitempty"`
	Provider                  Provider                  `json:"provider"`
	Maturity                  string                    `json:"maturity"`
	Version                   string                    `json:"version"`
	Icon                      []Icon                    `json:"icon,omitempty"`
	InstallModes              []InstallMode             `json:"installModes"`
	Install                   NamedInstallStrategy      `json:"install"`
	CustomResourceDefinitions CustomResourceDefinitions `json:"customresourcedefinitions"`
}

type Link struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

type Maintainer struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

type Provider struct {
	Name string `json:"name"`
	URL  string `json:"url,omitempty"`
}

type Icon struct {
	Base64Data string `json:"base64data"`
	MediaType  string `json:"mediatype"`
}

type InstallMode struct {
	Type      string `json:"type"`
	Supported bool   `json:"supported"`
}

type NamedInstallStrategy struct {
	Strategy string              `json:"strategy"`
	Spec     InstallStrategySpec `json:"spec"`
}

type InstallStrategySpec struct {
	ClusterPermissions []StrategyDeploymentPermissions `json:"clusterPermissions,omitempty"`
	Permissions        []StrategyDeploymentPermissions `json:"permissions,omitempty"`
	Deployments        []StrategyDeploymentSpec        `json:"deployments"`
}

type StrategyDeploymentPermissions struct {
	ServiceAccountName string       `json:"serviceAccountName"`
	Rules              []PolicyRule `json:"rules"`
}

type PolicyRule struct {
	APIGroups []string `json:"apiGroups"`
	Resources []string `json:"resources"`
	Verbs     []string `json:"verbs"`
}

type StrategyDeploymentSpec struct {
	Name  string            `json:"name"`
	Label map[string]string `json:"label,omitempty"`
	Spec  DeploymentSpec    `json:"spec"`
}

type DeploymentSpec struct {
	Replicas int32           `json:"replicas"`
	Selector *LabelSelector  `json:"selector"`
	Template PodTemplateSpec `json:"template"`
}

type LabelSelector struct {
	MatchLabels map[string]string `json:"matchLabels,omitempty"`
}

type PodTemplateSpec struct {
	Metadata PodMetadata `json:"metadata"`
	Spec     PodSpec     `json:"spec"`
}

type PodMetadata struct {
	Annotations map[string]string `json:"annotations,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
}

type PodSpec struct {
	Containers                    []Container         `json:"containers"`
	SecurityContext               *PodSecurityContext `json:"securityContext,omitempty"`
	ServiceAccountName            string              `json:"serviceAccountName,omitempty"`
	TerminationGracePeriodSeconds *int64              `json:"terminationGracePeriodSeconds,omitempty"`
}

type Container struct {
	Name                     string               `json:"name"`
	Image                    string               `json:"image"`
	ImagePullPolicy          string               `json:"imagePullPolicy,omitempty"`
	Command                  []string             `json:"command,omitempty"`
	Args                     []string             `json:"args,omitempty"`
	Env                      []EnvVar             `json:"env,omitempty"`
	Resources                ResourceRequirements `json:"resources,omitempty"`
	SecurityContext          *SecurityContext     `json:"securityContext,omitempty"`
	LivenessProbe            *Probe               `json:"livenessProbe,omitempty"`
	ReadinessProbe           *Probe               `json:"readinessProbe,omitempty"`
	TerminationMessagePolicy string               `json:"terminationMessagePolicy,omitempty"`
}

type EnvVar struct {
	Name  string `json:"name"`
	Value string `json:"value,omitempty"`
}

type ResourceRequirements struct {
	Limits   map[string]string `json:"limits,omitempty"`
	Requests map[string]string `json:"requests,omitempty"`
}

type SecurityContext struct {
	AllowPrivilegeEscalation *bool         `json:"allowPrivilegeEscalation,omitempty"`
	ReadOnlyRootFilesystem   *bool         `json:"readOnlyRootFilesystem,omitempty"`
	Capabilities             *Capabilities `json:"capabilities,omitempty"`
}

type Capabilities struct {
	Drop []string `json:"drop,omitempty"`
}

type PodSecurityContext struct {
	RunAsNonRoot   *bool           `json:"runAsNonRoot,omitempty"`
	SeccompProfile *SeccompProfile `json:"seccompProfile,omitempty"`
}

type SeccompProfile struct {
	Type string `json:"type"`
}

type Probe struct {
	HTTPGet             *HTTPGetAction `json:"httpGet,omitempty"`
	InitialDelaySeconds int32          `json:"initialDelaySeconds,omitempty"`
	PeriodSeconds       int32          `json:"periodSeconds,omitempty"`
}

type HTTPGetAction struct {
	Path string `json:"path"`
	Port int32  `json:"port"`
}

type CustomResourceDefinitions struct {
	Owned    []CRDDescription `json:"owned,omitempty"`
	Required []CRDDescription `json:"required,omitempty"`
}

type CRDDescription struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Kind        string `json:"kind"`
	DisplayName string `json:"displayName,omitempty"`
	Description string `json:"description,omitempty"`
}
