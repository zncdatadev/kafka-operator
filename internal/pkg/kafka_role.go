package pkg

import (
	"context"
	"fmt"

	"github.com/zncdatadev/kafka-operator/api/v1alpha1"
	authv1alpha1 "github.com/zncdatadev/operator-go/pkg/apis/authentication/v1alpha1"
	"github.com/zncdatadev/operator-go/pkg/client"
)

func NewClusterRole(cluster *v1alpha1.KafkaCluster, client *client.Client) *KafkaRole {
	return &KafkaRole{
		Cluster: cluster,
		client:  client,
	}
}

type KafkaRole struct {
	Cluster *v1alpha1.KafkaCluster
	client  *client.Client
}

func (k *KafkaRole) KerberosServiceName() string {
	return "kafka"
}

// ResolvedAuthenticationClasses holds the resolved authentication classes
type ResolvedAuthenticationClasses struct {
	Classes []authv1alpha1.AuthenticationClass
}

// FromReferences resolves authentication classes from references using the Kubernetes client
func (k *KafkaRole) FromReferences(
	ctx context.Context,
	authClasses []v1alpha1.KafkaAuthenticationSpec,
) (*ResolvedAuthenticationClasses, error) {
	if k.client == nil {
		return nil, fmt.Errorf("client is not initialized")
	}

	resolvedClasses := make([]authv1alpha1.AuthenticationClass, 0, len(authClasses))

	for _, authClass := range authClasses {
		if authClass.AuthenticationClass == "" {
			continue
		}

		// Resolve authentication class from the cluster
		authClassObj := &authv1alpha1.AuthenticationClass{}
		if err := k.client.GetWithOwnerNamespace(ctx, authClass.AuthenticationClass, authClassObj); err != nil {
			return nil, fmt.Errorf("failed to retrieve authentication class %s: %w",
				authClass.AuthenticationClass, err)
		}

		resolvedClasses = append(resolvedClasses, *authClassObj)
	}

	resolved := &ResolvedAuthenticationClasses{
		Classes: resolvedClasses,
	}

	return resolved.Validate()
}

// Validate validates the resolved authentication classes
// Currently errors out if:
// - More than one AuthenticationClass was provided
// - AuthenticationClass provider was not supported (only TLS and Kerberos are supported)
func (r *ResolvedAuthenticationClasses) Validate() (*ResolvedAuthenticationClasses, error) {
	if len(r.Classes) == 0 {
		return nil, fmt.Errorf("no authentication classes resolved")
	}

	if len(r.Classes) > 1 {
		return nil, fmt.Errorf("multiple authentication classes provided, only one is supported")
	}

	// Validate each authentication class provider
	for _, authClass := range r.Classes {
		if authClass.Spec.AuthenticationProvider == nil {
			return nil, fmt.Errorf("authentication class %s has no provider", authClass.Name)
		}

		provider := authClass.Spec.AuthenticationProvider

		// Check if provider is supported
		// Explicitly list each branch so new elements do not get overlooked
		if provider.TLS != nil || provider.Kerberos != nil {
			// TLS and Kerberos are supported
			continue
		}

		// Check for unsupported providers
		if provider.Static != nil {
			return nil, fmt.Errorf("authentication class %s uses Static provider which is not supported", authClass.Name)
		}
		if provider.LDAP != nil {
			return nil, fmt.Errorf("authentication class %s uses LDAP provider which is not supported", authClass.Name)
		}
		if provider.OIDC != nil {
			return nil, fmt.Errorf("authentication class %s uses OIDC provider which is not supported", authClass.Name)
		}

		return nil, fmt.Errorf("authentication class %s has unknown or unsupported provider", authClass.Name)
	}

	return r, nil
}

// GetKerberosSecretClass retrieves the Kerberos secret class from resolved authentication classes
func (k *KafkaRole) GetKerberosSecretClass(ctx context.Context) (string, error) {
	resolved, err := k.FromReferences(ctx, k.Cluster.Spec.ClusterConfig.Authentication)
	if err != nil {
		return "", fmt.Errorf("failed to resolve authentication classes: %w", err)
	}

	for _, authClass := range resolved.Classes {
		if authClass.Spec.AuthenticationProvider != nil &&
			authClass.Spec.AuthenticationProvider.Kerberos != nil &&
			authClass.Spec.AuthenticationProvider.Kerberos.KerberosStorageClass != "" {
			return authClass.Spec.AuthenticationProvider.Kerberos.KerberosStorageClass, nil
		}
	}

	return "", fmt.Errorf("no Kerberos authentication provider found")
}
