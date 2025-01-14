package controller

import (
	"github.com/zncdatadev/operator-go/pkg/builder"
	"github.com/zncdatadev/operator-go/pkg/client"
	"github.com/zncdatadev/operator-go/pkg/reconciler"
)

func NewServiceAccountReconciler(
	client *client.Client,
	saName string,
	opts ...builder.Option,
) reconciler.ResourceReconciler[*builder.GenericServiceAccountBuilder] {

	b := builder.NewGenericServiceAccountBuilder(
		client,
		saName,
		opts...,
	)
	return reconciler.NewGenericResourceReconciler(client, b)
}

// Note: enable in future,

// func NewClusterRoleReconciler(
// 	client *client.Client,
// 	name string,
// 	opts ...builder.Option,
// ) reconciler.ResourceReconciler[*builder.GenericClusterRoleBuilder] {
// 	b := builder.NewGenericClusterRoleBuilder(client, name)
// 	return reconciler.NewGenericResourceReconciler(client, b)
// }

// func NewClusterRoleBindingReconciler(
// 	client *client.Client,
// 	name string,
// 	opts ...builder.Option,
// ) reconciler.ResourceReconciler[*builder.GenericRoleBindingBuilder] {

// 	builder := builder.NewGenericRoleBindingBuilder(
// 		client,
// 		name,
// 		opts...,
// 	)
// 	return reconciler.NewGenericResourceReconciler(client, builder)
// }
