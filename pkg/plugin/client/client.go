package client

import (
	"context"
	"encoding/json"
	"fmt"

	api "github.com/openshift-virtualization/inflightoperations/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

var ifoGVR = schema.GroupVersionResource{
	Group:    "ifo.kubevirt.io",
	Version:  "v1alpha1",
	Resource: "inflightoperations",
}

// IFOClient wraps the dynamic Kubernetes client for IFO operations.
type IFOClient struct {
	dynamic dynamic.Interface
}

// NewFromKubeconfig creates an IFOClient from a kubeconfig path and context.
func NewFromKubeconfig(kubeconfig, kubeContext string) (*IFOClient, error) {
	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	if kubeconfig != "" {
		rules.ExplicitPath = kubeconfig
	}
	config, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		rules,
		&clientcmd.ConfigOverrides{CurrentContext: kubeContext},
	).ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("building kubeconfig: %w", err)
	}
	return newFromConfig(config)
}

func newFromConfig(config *rest.Config) (*IFOClient, error) {
	dyn, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("creating dynamic client: %w", err)
	}
	return &IFOClient{dynamic: dyn}, nil
}

// ListOptions controls server-side filtering for IFO queries.
type ListOptions struct {
	LabelSelector string
	FieldSelector string
}

// List returns IFOs matching the given options.
func (c *IFOClient) List(ctx context.Context, opts ListOptions) ([]api.InFlightOperation, error) {
	listOpts := metav1.ListOptions{
		LabelSelector: opts.LabelSelector,
		FieldSelector: opts.FieldSelector,
	}
	result, err := c.dynamic.Resource(ifoGVR).List(ctx, listOpts)
	if err != nil {
		return nil, fmt.Errorf("listing IFOs: %w", err)
	}
	return convertList(result.Items)
}

// Get returns a single IFO by name.
func (c *IFOClient) Get(ctx context.Context, name string) (*api.InFlightOperation, error) {
	result, err := c.dynamic.Resource(ifoGVR).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("getting IFO %q: %w", name, err)
	}
	return convert(result)
}

func convert(u *unstructured.Unstructured) (*api.InFlightOperation, error) {
	data, err := u.MarshalJSON()
	if err != nil {
		return nil, err
	}
	ifo := &api.InFlightOperation{}
	if err := json.Unmarshal(data, ifo); err != nil {
		return nil, err
	}
	return ifo, nil
}

func convertList(items []unstructured.Unstructured) ([]api.InFlightOperation, error) {
	result := make([]api.InFlightOperation, 0, len(items))
	for i := range items {
		ifo, err := convert(&items[i])
		if err != nil {
			return nil, err
		}
		result = append(result, *ifo)
	}
	return result, nil
}
