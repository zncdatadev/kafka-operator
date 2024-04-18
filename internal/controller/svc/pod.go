package svc

import (
	"context"
	"fmt"
	kafkav1alpha1 "github.com/zncdata-labs/kafka-operator/api/v1alpha1"
	"github.com/zncdata-labs/kafka-operator/internal/common"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"maps"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"time"
)

var podServiceLog = ctrl.Log.WithName("pod-service")
var _ common.SinglePodServiceResourceType = &PodServiceReconciler{}

type PodServiceReconciler struct {
	common.MultiResourceReconciler[*kafkav1alpha1.KafkaCluster, *kafkav1alpha1.BrokersRoleGroupSpec]
	replicas  int32
	groupName string
}

func NewPodServiceReconciler(
	scheme *runtime.Scheme,
	instance *kafkav1alpha1.KafkaCluster,
	client client.Client,
	groupName string,
	labels map[string]string,
	mergedCfg *kafkav1alpha1.BrokersRoleGroupSpec,
	replicas int32) *PodServiceReconciler {

	podSvcLabels := make(map[string]string)
	maps.Copy(podSvcLabels, labels)
	maps.Copy(podSvcLabels, ScopeLabels())

	return &PodServiceReconciler{
		MultiResourceReconciler: *common.NewMultiConfigurationStyleReconciler(scheme, instance, client, groupName,
			podSvcLabels, mergedCfg),
		replicas:  replicas,
		groupName: groupName,
	}
}

func ScopeLabels() map[string]string {
	return map[string]string{
		kafkav1alpha1.GroupName + "/scope": "pod",
	}
}
func (p *PodServiceReconciler) Build(ctx context.Context) ([]common.ResourceBuilder, error) {
	return p.CreateServiceByReplicas(ctx)
}

// create service by replicas

func (p *PodServiceReconciler) CreateServiceByReplicas(ctx context.Context) ([]common.ResourceBuilder, error) {
	var builders []common.ResourceBuilder
	statefulSetName := createStatefulSetName(p.Instance.GetName(), p.GroupName)
	for i := int32(0); i < p.replicas; i++ {
		podName := fmt.Sprintf("%s-%d", statefulSetName, i)
		pod, err := p.GetPod(ctx, podName)
		if err != nil {
			podServiceLog.V(1).Info("failed to get pod for service owner", "podName", podName, "err", err)
		} else {
			svc, err := p.CreateSingleService(podName, pod, p.ServicePorts())
			if err != nil {
				return nil, err
			}
			builders = append(builders, svc)
		}
	}
	return builders, nil
}

// create single service

func (p *PodServiceReconciler) CreateSingleService(
	podName string,
	pod *corev1.Pod,
	servicePorts []corev1.ServicePort) (common.ResourceBuilder, error) {
	headlessType := common.Service
	svcType := corev1.ServiceTypeNodePort
	builder := common.NewServiceBuilder(
		podName,
		p.Instance.GetNamespace(),
		p.Labels,
		servicePorts,
	).SetClusterIP(&headlessType).SetType(&svcType).SetSelector(p.podSelector(podName))
	reconciler := common.NewGenericServiceReconciler(
		p.Scheme,
		p.Instance,
		p.Client,
		p.GroupName,
		p.Labels,
		p.MergedCfg,
		builder)
	reconciler.SetOwner(pod)
	return reconciler, nil
}

// podSelector pod service labels
func (p *PodServiceReconciler) podSelector(podName string) map[string]string {
	return map[string]string{
		"statefulset.kubernetes.io/pod-name": podName,
	}
}

// ScopeLabels scope labels

// ClientNodePort Node port for pod
func (p *PodServiceReconciler) ClientNodePort(idx int32) int32 {
	return kafkav1alpha1.PodSvcClientNodePortMin + idx
}

// internal node port

// InternalNodePort internal node port
func (p *PodServiceReconciler) InternalNodePort(idx int32) int32 {
	return kafkav1alpha1.PodSvcInternalNodePortMin + idx
}

func (p *PodServiceReconciler) ServicePorts() []corev1.ServicePort {
	return []corev1.ServicePort{
		{
			Name:       kafkav1alpha1.KafkaPortName,
			Port:       PodServiceClientPort,
			Protocol:   corev1.ProtocolTCP,
			TargetPort: intstr.FromString(kafkav1alpha1.KafkaPortName),
		},
		{
			Name:       kafkav1alpha1.InternalPortName,
			Port:       PodServiceInternalPort,
			Protocol:   corev1.ProtocolTCP,
			TargetPort: intstr.FromString(kafkav1alpha1.InternalPortName),
		},
	}
}

// GetPod get pod
func (p *PodServiceReconciler) GetPod(ctx context.Context, podName string) (*corev1.Pod, error) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: p.Instance.Namespace,
		},
	}
	resourceClient := common.NewResourceClient(ctx, p.Client, p.Instance.Namespace)
	err := resourceClient.Get(pod)
	if err != nil {
		podServiceLog.V(0).Info(fmt.Sprintf("failed to get pod %s", pod.Name))
		return nil, err
	}
	return pod, nil
}

// Validate 1.check length of pod service is eq statefulSets.replicas
func (p *PodServiceReconciler) Validate(ctx context.Context) (ctrl.Result, error) {
	// get service list by labels
	serviceList := &corev1.ServiceList{}
	labelSelector := labels.SelectorFromSet(ScopeLabels())
	listOps := &client.ListOptions{
		Namespace:     p.Instance.Namespace,
		LabelSelector: labelSelector,
	}
	err := p.Client.List(ctx, serviceList, listOps)
	if err != nil {
		podServiceLog.V(1).Info("failed to list service list", "labels", p.Labels)
		return ctrl.Result{}, err
	}

	if len(serviceList.Items) != int(p.replicas) {
		podServiceLog.V(1).Info("service list length not equal replicas,will requeue", "service count",
			len(serviceList.Items), "replicas", p.replicas, "labels", p.Labels)
		return ctrl.Result{RequeueAfter: time.Second * 5}, nil
	}
	podServiceLog.V(4).Info("pod service validate success", "service count", len(serviceList.Items), "labels", p.Labels)
	return ctrl.Result{}, nil
}
