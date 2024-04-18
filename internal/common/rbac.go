package common

import (
	"context"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type RbacRoleType string
type RbacRoleBindingType string
type VerbType string
type ResourceType string

const (
	//ServiceAccount sa

	ServiceAccount = "ServiceAccount"

	//role
	RbacRole        RbacRoleType = "Role"
	RbacClusterRole RbacRoleType = "ClusterRole"

	// role binding
	RoleBinding        RbacRoleBindingType = "RoleBinding"
	ClusterRoleBinding RbacRoleBindingType = "ClusterRoleBinding"

	//verbs
	VerbsAll   VerbType = "*"
	Create     VerbType = "create"
	Update     VerbType = "update"
	DeleteVerb VerbType = "delete"
	Get        VerbType = "get"
	List       VerbType = "list"
	Patch      VerbType = "patch"
	Watch      VerbType = "watch"

	//resource types
	ResourceAll     ResourceType = "*"
	ServiceAccounts ResourceType = "serviceaccounts"
	ConfigMaps      ResourceType = "configmaps"
	Pods            ResourceType = "pods"
	Services        ResourceType = "services"
	Secrets         ResourceType = "secrets"
	Namespaces      ResourceType = "namespaces"
	Events          ResourceType = "events"
	StatefulSets    ResourceType = "statefulsets"

	//api group
	ApiGroupALL = "*"
)

// GenericServiceAccountReconciler generci service account reconcile
type GenericServiceAccountReconciler[T client.Object, G any] struct {
	GeneralResourceStyleReconciler[T, G]
	saName    string
	nameSpace string
}

// NewServiceAccount new a ServiceAccountReconciler
func NewServiceAccount[T client.Object](
	scheme *runtime.Scheme,
	instance T,
	client client.Client,
	mergedLabels map[string]string,
	mergedCfg any,
	saName string,
	nameSpace string,
) *GenericServiceAccountReconciler[T, any] {
	return &GenericServiceAccountReconciler[T, any]{
		GeneralResourceStyleReconciler: *NewGeneraResourceStyleReconciler(
			scheme,
			instance,
			client,
			"",
			mergedLabels,
			mergedCfg,
		),
		saName:    saName,
		nameSpace: nameSpace,
	}
}

// Build implements the ResourceBuilder interface
func (r *GenericServiceAccountReconciler[T, G]) Build(_ context.Context) (client.Object, error) {
	saToken := true
	return &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      r.saName,
			Namespace: r.nameSpace,
			Labels:    r.Labels,
		},
		AutomountServiceAccountToken: &saToken,
	}, nil
}

// generic role reconciler

// GenericRoleReconciler generic role reconciler
type GenericRoleReconciler[T client.Object, G any] struct {
	GeneralResourceStyleReconciler[T, G]
	namespace string
	roleName  string
	roleType  RbacRoleType
	verbs     []string
	apiGroups []string
	resources []string
}

// NewRole new a RoleReconciler
func NewRole[T client.Object](
	scheme *runtime.Scheme,
	instance T,
	client client.Client,
	groupName string,
	mergedLabels map[string]string,
	mergedCfg any,

	roleType RbacRoleType,
	roleName string,
	verbs []VerbType,
	apiGroups []string,
	resources []ResourceType,
	namespace string,
) *GenericRoleReconciler[T, any] {
	return &GenericRoleReconciler[T, any]{
		GeneralResourceStyleReconciler: *NewGeneraResourceStyleReconciler(
			scheme,
			instance,
			client,
			groupName,
			mergedLabels,
			mergedCfg,
		),
		namespace: namespace,
		roleName:  roleName,
		roleType:  roleType,
		verbs:     verbsString(verbs),
		apiGroups: apiGroups,
		resources: resourceTypesString(resources),
	}
}

// Build implements the ResourceBuilder interface
func (r *GenericRoleReconciler[T, G]) Build(_ context.Context) (client.Object, error) {
	rules := []v1.PolicyRule{
		{
			Verbs:     r.verbs,
			APIGroups: r.apiGroups,
			Resources: r.resources,
		},
	}
	// Note: cluster level resource must not add namespace field
	if r.roleType == RbacClusterRole {
		return &v1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{Name: r.roleName, Labels: r.Labels},
			Rules:      rules,
		}, nil
	}
	return &v1.Role{
		ObjectMeta: metav1.ObjectMeta{Name: r.roleName, Namespace: r.namespace, Labels: r.Labels},
		Rules:      rules,
	}, nil

}

// generic role bindings

// GenericRoleBindingReconciler generic role binding reconciler
type GenericRoleBindingReconciler[T client.Object, G any] struct {
	GeneralResourceStyleReconciler[T, G]
	namespace       string
	roleBindingType RbacRoleBindingType
	roleBindingName string
	apiGroup        string
	roleName        string
	saName          string
}

// NewRoleBinding new a RoleBindingReconciler
func NewRoleBinding[T client.Object](
	scheme *runtime.Scheme,
	instance T,
	client client.Client,
	groupName string,
	mergedLabels map[string]string,
	mergedCfg any,

	apiGroup string,
	roleBindType RbacRoleBindingType,
	roleBindingName string,
	roleName string,
	saName string,
	namespace string,
) *GenericRoleBindingReconciler[T, any] {
	return &GenericRoleBindingReconciler[T, any]{
		GeneralResourceStyleReconciler: *NewGeneraResourceStyleReconciler(
			scheme,
			instance,
			client,
			groupName,
			mergedLabels,
			mergedCfg,
		),
		namespace:       namespace,
		apiGroup:        apiGroup,
		roleBindingType: roleBindType,
		roleBindingName: roleBindingName,
		roleName:        roleName,
		saName:          saName,
	}
}

// Build implements the ResourceBuilder interface
func (r *GenericRoleBindingReconciler[T, G]) Build(_ context.Context) (client.Object, error) {
	subjects := []v1.Subject{
		{
			Kind:      ServiceAccount,
			Name:      r.saName,
			Namespace: r.namespace,
		},
	}

	// Note: cluster level resource must not add namespace field
	if r.roleBindingType == ClusterRoleBinding {
		return &v1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{Name: r.roleBindingName, Labels: r.Labels},
			Subjects:   subjects,
			RoleRef: v1.RoleRef{
				APIGroup: r.apiGroup,
				Kind:     string(RbacClusterRole),
				Name:     r.roleName,
			},
		}, nil
	}
	return &v1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      r.roleBindingName,
			Namespace: r.namespace,
			Labels:    r.Labels,
		},
		Subjects: subjects,
		RoleRef: v1.RoleRef{
			APIGroup: r.apiGroup,
			Kind:     string(RbacRole),
			Name:     r.roleName,
		},
	}, nil
}

func verbsString(enums []VerbType) []string {
	verbs := make([]string, 0)
	for _, v := range enums {
		verbs = append(verbs, string(v))
	}
	return verbs
}

func resourceTypesString(enums []ResourceType) []string {
	resources := make([]string, 0)
	for _, v := range enums {
		resources = append(resources, string(v))
	}
	return resources
}
