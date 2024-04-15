package svc

import (
	"context"
	"fmt"
	kafkav1alpha1 "github.com/zncdata-labs/kafka-operator/api/v1alpha1"
	"github.com/zncdata-labs/kafka-operator/internal/common"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var podServiceLog = ctrl.Log.WithName("pod-service")
var _ common.SinglePodServiceResourceType = &PodServiceReconciler{}

type PodServiceReconciler struct {
	common.GeneralResourceStyleReconciler[*kafkav1alpha1.KafkaCluster, *kafkav1alpha1.BrokersRoleGroupSpec]
	replicas int32
}

func NewPodServiceReconciler(
	scheme *runtime.Scheme,
	instance *kafkav1alpha1.KafkaCluster,
	client client.Client,
	groupName string,
	roleLabels map[string]string,
	mergedCfg *kafkav1alpha1.BrokersRoleGroupSpec,
	replicas int32) *PodServiceReconciler {
	return &PodServiceReconciler{
		GeneralResourceStyleReconciler: *common.NewGeneraResourceStyleReconciler(scheme, instance, client, groupName,
			roleLabels, mergedCfg),
		replicas: replicas,
	}
}

func (p PodServiceReconciler) Build(ctx context.Context) ([]common.ResourceBuilder, error) {
	return p.CreateServiceByReplicas(ctx)
}

// create service by replicas

func (p PodServiceReconciler) CreateServiceByReplicas(ctx context.Context) ([]common.ResourceBuilder, error) {
	var builders []common.ResourceBuilder
	for i := int32(0); i < p.replicas; i++ {
		podName := fmt.Sprintf("%s-%d", p.Instance.GetName(), i)
		svc, err := p.CreateSingleService(ctx, podName, p.ServicePorts(i))
		if err != nil {
			return nil, err
		}
		builders = append(builders, svc)
	}
	return builders, nil
}

// create single service

func (p PodServiceReconciler) CreateSingleService(
	ctx context.Context,
	podName string,
	servicePorts []corev1.ServicePort) (common.ResourceBuilder, error) {
	headlessType := common.Service
	svcType := corev1.ServiceTypeNodePort
	builder := common.NewServiceBuilder(
		podName,
		p.Instance.GetNamespace(),
		p.MergedLabels,
		servicePorts,
	).SetClusterIP(&headlessType).SetType(&svcType)

	reconciler := common.NewGenericServiceReconciler(
		p.Scheme,
		p.Instance,
		p.Client,
		p.GroupName,
		p.MergedLabels,
		p.MergedCfg,
		builder)
	// get pod by pod name
	// if pod not exist return error, else set the pod to owner of reconciler
	pod, err := p.GetPod(ctx, podName)
	if err != nil {
		return nil, err
	}
	reconciler.SetOwner(pod)
	return reconciler, nil
}

// ClientNodePort Node port for pod
func (p PodServiceReconciler) ClientNodePort(idx int32) int32 {
	return kafkav1alpha1.PodSvcClientNodePortMin + idx
}

// internal node port

// InternalNodePort internal node port
func (p PodServiceReconciler) InternalNodePort(idx int32) int32 {
	return kafkav1alpha1.PodSvcInternalNodePortMin + p.replicas + idx
}

// service ports

func (p PodServiceReconciler) ServicePorts(idx int32) []corev1.ServicePort {
	return []corev1.ServicePort{
		{
			Name:       kafkav1alpha1.KafkaPortName,
			Port:       PodServiceClientPort,
			Protocol:   corev1.ProtocolTCP,
			TargetPort: intstr.FromString(kafkav1alpha1.KafkaPortName),
			NodePort:   p.ClientNodePort(idx),
		},
		{
			Name:       kafkav1alpha1.InternalPortName,
			Port:       PodServiceInternalPort,
			Protocol:   corev1.ProtocolTCP,
			TargetPort: intstr.FromString(kafkav1alpha1.InternalPortName),
			NodePort:   p.InternalNodePort(idx),
		},
	}
}

// get pod

func (p PodServiceReconciler) GetPod(ctx context.Context, podName string) (*corev1.Pod, error) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: p.Instance.Namespace,
		},
	}
	resourceClient := common.NewResourceClient(ctx, p.Client, p.Instance.Namespace)
	err := resourceClient.Get(pod)
	if err != nil {
		podServiceLog.Error(err, fmt.Sprintf("failed to get pod %s", pod.Name))
		return nil, err
	}
	return pod, nil
}
