package activity

import (
	"context"
	"fmt"

	"launchs/shared/database"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// ServiceActivity は Kubernetes Service の作成・削除を担当します。
type ServiceActivity struct{}

// CreateOrUpdate は Service を作成または更新します。
func (a *ServiceActivity) CreateOrUpdate(ctx context.Context, spec ServiceSpec) error {
	k8s := database.K8sClientset

	ports := make([]corev1.ServicePort, 0, len(spec.Ports))
	for _, p := range spec.Ports {
		proto := corev1.ProtocolTCP
		if p.Protocol == "UDP" {
			proto = corev1.ProtocolUDP
		}
		ports = append(ports, corev1.ServicePort{
			Port:       int32(p.Port),
			TargetPort: intstr.FromInt(p.Port),
			Protocol:   proto,
		})
	}

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      spec.Name,
			Namespace: spec.Namespace,
			Labels:    map[string]string{"launchs-managed": "true"},
		},
		Spec: corev1.ServiceSpec{
			Selector: spec.SelectorLabels,
			Ports:    ports,
			Type:     corev1.ServiceTypeClusterIP,
		},
	}

	existing, err := k8s.CoreV1().Services(spec.Namespace).Get(ctx, spec.Name, metav1.GetOptions{})
	if err != nil {
		if !k8serrors.IsNotFound(err) {
			return fmt.Errorf("Service 取得エラー: %w", err)
		}
		_, err = k8s.CoreV1().Services(spec.Namespace).Create(ctx, svc, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("Service 作成エラー: %w", err)
		}
		return nil
	}

	svc.ResourceVersion = existing.ResourceVersion
	svc.Spec.ClusterIP = existing.Spec.ClusterIP // ClusterIP は不変なので引き継ぐ
	_, err = k8s.CoreV1().Services(spec.Namespace).Update(ctx, svc, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("Service 更新エラー: %w", err)
	}
	return nil
}

// Delete は Service を削除します。
func (a *ServiceActivity) Delete(ctx context.Context, namespace, name string) error {
	k8s := database.K8sClientset
	err := k8s.CoreV1().Services(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil && !k8serrors.IsNotFound(err) {
		return fmt.Errorf("Service 削除エラー %s/%s: %w", namespace, name, err)
	}
	return nil
}
