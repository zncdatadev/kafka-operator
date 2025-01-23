package util

import (
	"fmt"

	"github.com/zncdatadev/operator-go/pkg/constants"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

type ListenerReference struct {
	ListenerClass string
	ListenerName  string
}

func (r *ListenerReference) ToAnnotation() (*Annotation, error) {
	if r.ListenerClass != "" {
		return NewAnnotation(constants.AnnotationListenersClass, r.ListenerClass)
	}
	if r.ListenerName != "" {
		return NewAnnotation(constants.AnnotationListenerName, r.ListenerName)
	}
	return nil, fmt.Errorf("either ListenerClass or ListenerName must be set")
}

type Annotation struct {
	Key   string
	Value string
}

func NewAnnotation(key, value string) (*Annotation, error) {
	if key == "" {
		return nil, fmt.Errorf("annotation key cannot be empty")
	}
	if value == "" {
		return nil, fmt.Errorf("annotation value cannot be empty")
	}
	return &Annotation{
		Key:   key,
		Value: value,
	}, nil
}

type ListenerOperatorVolumeSourceBuilder struct {
	listenerClass ListenerReference
	labels        map[string]string
}

func NewListenerOperatorVolumeSourceBuilder(
	listenerReference *ListenerReference,
	labels map[string]string,
) *ListenerOperatorVolumeSourceBuilder {
	return &ListenerOperatorVolumeSourceBuilder{
		listenerClass: *listenerReference,
		labels:        labels,
	}
}

func (b *ListenerOperatorVolumeSourceBuilder) buildSpec() corev1.PersistentVolumeClaimSpec {
	return corev1.PersistentVolumeClaimSpec{
		StorageClassName: ptr.To(constants.ListenerStorageClass),
		Resources: corev1.VolumeResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceStorage: resource.MustParse("10Mi"),
			},
		},
		VolumeMode:  ptr.To(corev1.PersistentVolumeFilesystem), // must set, otherwise it will be into patch reconcile
		AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
	}
}

func (b *ListenerOperatorVolumeSourceBuilder) BuildEphemeral() (*corev1.EphemeralVolumeSource, error) {
	listenerReferenceAnnotation, err := b.listenerClass.ToAnnotation()
	if err != nil {
		return nil, fmt.Errorf("failed to get listener reference annotation: %w", err)
	}

	return &corev1.EphemeralVolumeSource{
		VolumeClaimTemplate: &corev1.PersistentVolumeClaimTemplate{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{listenerReferenceAnnotation.Key: listenerReferenceAnnotation.Value},
				Labels:      b.labels,
			},
			Spec: b.buildSpec(),
		},
	}, nil
}

func (b *ListenerOperatorVolumeSourceBuilder) BuildPVC(name string) (*corev1.PersistentVolumeClaim, error) {
	listenerReferenceAnnotation, err := b.listenerClass.ToAnnotation()
	if err != nil {
		return nil, fmt.Errorf("failed to get listener reference annotation: %w", err)
	}

	return &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Annotations: map[string]string{listenerReferenceAnnotation.Key: listenerReferenceAnnotation.Value},
			Labels:      b.labels,
		},
		Spec: b.buildSpec(),
	}, nil
}
