package container

import (
	"fmt"
	"github.com/zncdata-labs/kafka-operator/api/v1alpha1"
	"github.com/zncdata-labs/kafka-operator/internal/common"
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
	return string(FetchNodePort)
}

func (d *FetchNodePortContainerBuilder) ContainerEnv() []corev1.EnvVar {
	return []corev1.EnvVar{
		{
			Name: EnvPodName,
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
			MountPath: NodePortMountPath,
		},
	}
}

// CommandArgs command args
// ex:get service "$POD_NAME" -o jsonpath='{.spec.ports[?(@.name=="kafka")].nodePort}' | tee /zncdata/tmp/kafka_nodeport
func (d *FetchNodePortContainerBuilder) CommandArgs() []string {
	args := fmt.Sprintf("get service \"$%s\" -o jsonpath='{.spec.ports[?(@.name==\"%s\")].nodePort}' | tee %s/%s",
		EnvPodName, v1alpha1.KafkaPortName, NodePortMountPath, NodePortFileName)
	return []string{args}
}
