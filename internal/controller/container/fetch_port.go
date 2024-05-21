package container

import (
	"fmt"

	"github.com/zncdatadev/kafka-operator/api/v1alpha1"
	"github.com/zncdatadev/kafka-operator/internal/common"
	corev1 "k8s.io/api/core/v1"
)

type FetchNodePortContainerBuilder struct {
	common.ContainerBuilder
}

func NewFetchNodePortContainerBuilder() *FetchNodePortContainerBuilder {
	return &FetchNodePortContainerBuilder{
		ContainerBuilder: *common.NewContainerBuilder("bitnami/kubectl", corev1.PullIfNotPresent),
	}
}

func (d *FetchNodePortContainerBuilder) ContainerName() string {
	return string(common.FetchNodePort)
}

func (d *FetchNodePortContainerBuilder) ContainerEnv() []corev1.EnvVar {
	return []corev1.EnvVar{
		{
			Name: common.EnvPodName,
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.name"},
			},
		},
	}
}

func (d *FetchNodePortContainerBuilder) VolumeMount() []corev1.VolumeMount {
	return []corev1.VolumeMount{
		{
			Name:      NodePortVolumeName(),
			MountPath: common.NodePortMountPath,
		},
	}
}

func (d *FetchNodePortContainerBuilder) Command() []string {
	cmdTpl := `SERVICE_NAME=$%s
echo "Service Name : $SERVICE_NAME"

LOCATION="%s/%s"
if kubectl get svc $SERVICE_NAME; then
    if kubectl get service $SERVICE_NAME -o jsonpath='{.spec.ports[?(@.name=="%s")].nodePort}' > $LOCATION; then
        echo "Service $SERVICE_NAME nodeport: $(cat $LOCATION) saved to $LOCATION"
        exit 0
    else
        echo "Service $SERVICE_NAME not found or does not have a NodePort configured." >&2
        exit 1
    fi
else
    echo "Service $SERVICE_NAME not found." >&2
    exit 1
fi
`
	return []string{"sh", "-c", fmt.Sprintf(cmdTpl, common.EnvPodName, common.NodePortMountPath, common.NodePortFileName,
		v1alpha1.KafkaPortName)}
}
