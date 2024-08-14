package util

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type SecretVolumeBuilder struct {
	VolumeName string

	annotaions map[string]string
}

// add annotation
func (s *SecretVolumeBuilder) AddAnnotation(key, value string) *SecretVolumeBuilder {
	if s.annotaions == nil {
		s.annotaions = make(map[string]string)
	}
	s.annotaions[key] = value
	return s
}

// set annotaions
func (s *SecretVolumeBuilder) SetAnnotations(annotations map[string]string) *SecretVolumeBuilder {
	s.annotaions = annotations
	return s
}

// Build
func (s *SecretVolumeBuilder) Build() corev1.Volume {
	return corev1.Volume{
		Name: s.VolumeName,
		VolumeSource: corev1.VolumeSource{
			Ephemeral: &corev1.EphemeralVolumeSource{
				VolumeClaimTemplate: &corev1.PersistentVolumeClaimTemplate{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: s.annotaions,
					},
					Spec: corev1.PersistentVolumeClaimSpec{
						AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
						StorageClassName: func() *string {
							cs := "secrets.zncdata.dev"
							return &cs
						}(),
						VolumeMode: func() *corev1.PersistentVolumeMode { v := corev1.PersistentVolumeFilesystem; return &v }(),
						Resources: corev1.VolumeResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceStorage: resource.MustParse("10Mi"),
							},
						},
					},
				},
			},
		},
	}
}
