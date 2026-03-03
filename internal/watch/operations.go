package watch

import (
	"context"
	"fmt"
	"slices"

	api "github.com/ifo-operator/inflightoperations/api/v1alpha1"
	liberr "github.com/ifo-operator/inflightoperations/lib/error"
	"github.com/ifo-operator/inflightoperations/lib/logging"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8slabels "k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Operations struct {
	client client.Client
	log    logging.LevelLogger
}

func (r *Operations) DeleteAll(ctx context.Context, subject *api.Subject) (err error) {
	err = r.client.DeleteAllOf(
		ctx,
		&api.InFlightOperation{},
		&client.DeleteAllOfOptions{
			ListOptions: client.ListOptions{
				LabelSelector: k8slabels.SelectorFromSet(r.subjectLabel(subject)),
			},
		})
	if err != nil {
		return
	}
	return
}

func (r *Operations) Find(ctx context.Context, subject *api.Subject, operation string) (op *api.InFlightOperation, found bool, err error) {
	list := &api.InFlightOperationList{}
	err = r.client.List(ctx, list, &client.ListOptions{
		LabelSelector: k8slabels.SelectorFromSet(r.operationLabels(subject, operation)),
	})
	if err != nil {
		return
	}
	slices.SortFunc(list.Items, func(i, j api.InFlightOperation) int {
		return i.CreationTimestamp.Compare(j.CreationTimestamp.Time) * -1
	})
	if len(list.Items) == 0 {
		return
	}
	op = &list.Items[0]
	if !list.Items[0].PastDebounceThreshold() {
		r.log.Info("Found active InFlightOperation resource.", "name", op.Name)
		found = true
	}
	return
}

func (r *Operations) List(ctx context.Context, subject *api.Subject) (list *api.InFlightOperationList, err error) {
	list = &api.InFlightOperationList{}
	err = r.client.List(ctx, list, &client.ListOptions{
		LabelSelector: k8slabels.SelectorFromSet(r.subjectLabel(subject)),
	})
	if err != nil {
		return
	}
	return
}

func (r *Operations) Build(subject *api.Subject, operation string) (op *api.InFlightOperation) {
	op = &api.InFlightOperation{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("%s-", subject.GetName()),
			Labels:       r.operationLabels(subject, operation),
		},
		Spec: api.InFlightOperationSpec{
			Operation: operation,
			Subject: api.SubjectReference{
				APIVersion: subject.GetAPIVersion(),
				Kind:       subject.GetKind(),
				Name:       subject.GetName(),
				Namespace:  subject.GetNamespace(),
				UID:        string(subject.GetUID()),
			},
		},
	}
	return
}

func (r *Operations) Ensure(ctx context.Context, op *api.InFlightOperation) (out *api.InFlightOperation, err error) {
	list := &api.InFlightOperationList{}
	err = r.client.List(ctx, list, &client.ListOptions{
		LabelSelector: k8slabels.SelectorFromSet(op.Labels),
	})
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	slices.SortFunc(list.Items, func(i, j api.InFlightOperation) int {
		return i.CreationTimestamp.Compare(j.CreationTimestamp.Time) * -1
	})
	if len(list.Items) == 0 || list.Items[0].PastDebounceThreshold() {
		err = r.client.Create(ctx, op)
		if err != nil {
			err = liberr.Wrap(err)
			return
		}
		r.log.Info("Created InFlightOperation resource.", "name", op.Name)
		out = op
	} else {
		out = &list.Items[0]
	}
	return
}

func (r *Operations) subjectLabel(subject *api.Subject) map[string]string {
	return map[string]string{
		"subjectUID":       string(subject.GetUID()),
		"subjectNamespace": subject.GetNamespace(),
	}
}

func (r *Operations) operationLabels(subject *api.Subject, operation string) map[string]string {
	return map[string]string{
		"subjectUID":       string(subject.GetUID()),
		"subjectNamespace": subject.GetNamespace(),
		"operation":        operation,
	}
}
