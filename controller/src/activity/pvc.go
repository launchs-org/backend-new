package activity

import (
	"context"
	"fmt"

	"launchs/shared/database"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PVCActivity は PersistentVolumeClaim の作成・削除を担当します。
type PVCActivity struct{}

// PVCCreate は PVC を作成します。既に存在する場合は何もしません（冪等）。
func (a *PVCActivity) PVCCreate(ctx context.Context, spec PVCSpec) error {
	k8s := database.K8sClientset

	storageSize := spec.StorageSize
	if storageSize == "" {
		storageSize = "1Gi"
	}

	storageClass := "standard"
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      spec.Name,
			Namespace: spec.Namespace,
			Labels:    map[string]string{"launchs-managed": "true"},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			StorageClassName: &storageClass,
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse(storageSize),
				},
			},
		},
	}

	_, err := k8s.CoreV1().PersistentVolumeClaims(spec.Namespace).Create(ctx, pvc, metav1.CreateOptions{})
	if err != nil && !k8serrors.IsAlreadyExists(err) {
		return fmt.Errorf("PVC 作成エラー %s/%s: %w", spec.Namespace, spec.Name, err)
	}
	return nil
}

// PVCDelete は PVC を削除します。
func (a *PVCActivity) PVCDelete(ctx context.Context, namespace, name string) error {
	k8s := database.K8sClientset
	err := k8s.CoreV1().PersistentVolumeClaims(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil && !k8serrors.IsNotFound(err) {
		return fmt.Errorf("PVC 削除エラー %s/%s: %w", namespace, name, err)
	}
	return nil
}

