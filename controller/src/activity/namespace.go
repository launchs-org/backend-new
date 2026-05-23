package activity

import (
	"context"
	"fmt"

	"launchs/shared/database"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NamespaceActivity は Kubernetes Namespace の作成・削除を担当します。
type NamespaceActivity struct{}

// NamespaceCreate は Namespace と NetworkPolicy を作成します。
// launchs-managed ラベルを付与し、Traefik / cloudflared からのトラフィックのみ許可します。
func (a *NamespaceActivity) NamespaceCreate(ctx context.Context, namespace, projectID string) error {
	k8s := database.K8sClientset

	nsSpec := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace,
			Labels: map[string]string{
				"managed-by": "launchs",
				"project-id": projectID,
			},
		},
	}

	_, err := k8s.CoreV1().Namespaces().Create(ctx, nsSpec, metav1.CreateOptions{})
	if err != nil && !k8serrors.IsAlreadyExists(err) {
		return fmt.Errorf("namespace 作成エラー %s: %w", namespace, err)
	}

	// Traefik / cloudflared / 同一Namespace からの通信のみ許可する NetworkPolicy を適用します。
	netPol := &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "allow-traefik-cloudflared-local",
			Namespace: namespace,
		},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{},
			PolicyTypes: []networkingv1.PolicyType{
				networkingv1.PolicyTypeIngress,
				networkingv1.PolicyTypeEgress,
			},
			Ingress: []networkingv1.NetworkPolicyIngressRule{
				{
					From: []networkingv1.NetworkPolicyPeer{
						{NamespaceSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{"kubernetes.io/metadata.name": "traefik"},
						}},
						{NamespaceSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{"kubernetes.io/metadata.name": "cloudflared"},
						}},
						{NamespaceSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{"kubernetes.io/metadata.name": namespace},
						}},
					},
				},
			},
			Egress: []networkingv1.NetworkPolicyEgressRule{{}},
		},
	}

	_, err = k8s.NetworkingV1().NetworkPolicies(namespace).Create(ctx, netPol, metav1.CreateOptions{})
	if err != nil && !k8serrors.IsAlreadyExists(err) {
		return fmt.Errorf("NetworkPolicy 作成エラー %s: %w", namespace, err)
	}

	return nil
}

// NamespaceDelete は Namespace を削除します（配下のリソースも一括削除されます）。
func (a *NamespaceActivity) NamespaceDelete(ctx context.Context, namespace string) error {
	k8s := database.K8sClientset
	err := k8s.CoreV1().Namespaces().Delete(ctx, namespace, metav1.DeleteOptions{})
	if err != nil && !k8serrors.IsNotFound(err) {
		return fmt.Errorf("Namespace 削除エラー %s: %w", namespace, err)
	}
	return nil
}
