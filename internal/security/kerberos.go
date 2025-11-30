package security

import (
	"context"
	"fmt"

	kafkav1alpha1 "github.com/zncdatadev/kafka-operator/api/v1alpha1"
	"github.com/zncdatadev/kafka-operator/internal/pkg"
	"github.com/zncdatadev/operator-go/pkg/constants"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func NewKerberosAuthentication(role *pkg.KafkaRole) *KerberosAuthentication {
	return &KerberosAuthentication{
		Role: role,
	}
}

type KerberosAuthentication struct {
	Role *pkg.KafkaRole
}

// Get volume mount
func (k *KerberosAuthentication) GetVolumeMount() []corev1.VolumeMount {
	return []corev1.VolumeMount{
		{
			Name:      kafkav1alpha1.KubedoopKerberosName,
			MountPath: kafkav1alpha1.KubedoopKerberosDir,
		},
	}
}

// Get envs
func (k *KerberosAuthentication) GetEnvs() []corev1.EnvVar {
	return []corev1.EnvVar{
		{
			Name:  "KRB5_CONFIG",
			Value: kafkav1alpha1.KubedoopKerberosKrb5Path,
		},
		{
			Name:  "KAFKA_OPTS",
			Value: fmt.Sprintf("-Djava.security.krb5.conf=%s", kafkav1alpha1.KubedoopKerberosKrb5Path),
		},
	}
}

// Get Volumes
func (k *KerberosAuthentication) GetVolumes(ctx context.Context) ([]corev1.Volume, error) {

	listenerScopes := []string{kafkav1alpha1.KubedoopListenerBroker, kafkav1alpha1.KubedoopListenerBootstrap}
	scopeStr := ""
	for _, s := range listenerScopes {
		scopeStr += "," + "listener-volume=" + s
	}

	kerberosSecretClass, err := k.Role.GetKerberosSecretClass(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get Kerberos secret class: %w", err)
	}

	return []corev1.Volume{
		{
			Name: kafkav1alpha1.KubedoopKerberosName,
			VolumeSource: corev1.VolumeSource{
				Ephemeral: &corev1.EphemeralVolumeSource{
					VolumeClaimTemplate: &corev1.PersistentVolumeClaimTemplate{
						ObjectMeta: metav1.ObjectMeta{
							Annotations: map[string]string{
								constants.AnnotationSecretsClass:                kerberosSecretClass,
								constants.AnnotationSecretsScope:                scopeStr[1:], // remove leading ","
								constants.AnnotationSecretsKerberosServiceNames: k.Role.KerberosServiceName(),
							},
						},
						Spec: corev1.PersistentVolumeClaimSpec{
							AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
							StorageClassName: func() *string {
								cs := "secrets.kubedoop.dev"
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
		},
	}, nil
}
