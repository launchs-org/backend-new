package activity

import (
	"context"
	"fmt"

	"launchs/shared/database"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// IngressActivity は Traefik IngressRoute CRD の作成・削除を担当します。
type IngressActivity struct{}

// traefik IngressRoute の GVR
var ingressRouteGVR = schema.GroupVersionResource{
	Group:    "traefik.io",
	Version:  "v1alpha1",
	Resource: "ingressroutes",
}

// CreateOrUpdate は Traefik IngressRoute を作成または更新します。
// unstructured で CRD を操作し、Traefik に HTTP ルートを登録します。
func (a *IngressActivity) CreateOrUpdate(ctx context.Context, spec IngressSpec) error {
	dynClient := database.K8sDynamicClient

	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "traefik.io/v1alpha1",
			"kind":       "IngressRoute",
			"metadata": map[string]interface{}{
				"name":      spec.Name,
				"namespace": spec.Namespace,
				"labels": map[string]interface{}{
					"launchs-managed": "true",
				},
			},
			"spec": map[string]interface{}{
				"entryPoints": []interface{}{"websecure"},
				"routes": []interface{}{
					map[string]interface{}{
						"match": fmt.Sprintf("Host(`%s`)", spec.Host),
						"kind":  "Rule",
						"services": []interface{}{
							map[string]interface{}{
								"name": spec.ServiceName,
								"port": int64(spec.Port),
							},
						},
					},
				},
				"tls": map[string]interface{}{
					"certResolver": "default",
				},
			},
		},
	}

	_, err := dynClient.Resource(ingressRouteGVR).Namespace(spec.Namespace).Get(ctx, spec.Name, metav1.GetOptions{})
	if err != nil {
		if !k8serrors.IsNotFound(err) {
			return fmt.Errorf("IngressRoute 取得エラー: %w", err)
		}
		_, err = dynClient.Resource(ingressRouteGVR).Namespace(spec.Namespace).Create(ctx, obj, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("IngressRoute 作成エラー: %w", err)
		}
		return nil
	}

	_, err = dynClient.Resource(ingressRouteGVR).Namespace(spec.Namespace).Update(ctx, obj, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("IngressRoute 更新エラー: %w", err)
	}
	return nil
}

// Delete は IngressRoute を削除します。
func (a *IngressActivity) Delete(ctx context.Context, namespace, name string) error {
	dynClient := database.K8sDynamicClient
	err := dynClient.Resource(ingressRouteGVR).Namespace(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil && !k8serrors.IsNotFound(err) {
		return fmt.Errorf("IngressRoute 削除エラー %s/%s: %w", namespace, name, err)
	}
	return nil
}
