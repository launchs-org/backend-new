package utils

import (
	"context"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/client-go/kubernetes"
)
func IsDeploymentReady(dep *appsv1.Deployment) bool {
    // Generation が反映されていない場合はまだ処理中
    if dep.Generation > dep.Status.ObservedGeneration {
        return false
    }

    desired := int32(1)
    if dep.Spec.Replicas != nil {
        desired = *dep.Spec.Replicas
    }

    // 0スケール時は特別扱い
    if desired == 0 {
        return dep.Status.Replicas == 0
    }

    return dep.Status.UpdatedReplicas == desired &&
        dep.Status.ReadyReplicas == desired &&
        dep.Status.AvailableReplicas == desired &&
        dep.Status.UnavailableReplicas == 0
}

func WaitForDeploymentReady(ctx context.Context, k8s kubernetes.Interface, namespace, name string, timeout time.Duration) error {
    deadline := time.Now().Add(timeout)
    ticker := time.NewTicker(5 * time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return fmt.Errorf("context cancelled while waiting for deployment %s/%s: %w", namespace, name, ctx.Err())
        case <-ticker.C:
            if time.Now().After(deadline) {
                // タイムアウト時に最後の状態を取得して詳細エラーを返す
                dep, err := k8s.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
                if err != nil {
                    return fmt.Errorf("deployment %s/%s timed out and failed to get status: %w", namespace, name, err)
                }
                return fmt.Errorf(
                    "deployment %s/%s timed out after %s: desired=%d, ready=%d, updated=%d, available=%d, unavailable=%d",
                    namespace, name, timeout,
                    func() int32 {
                        if dep.Spec.Replicas != nil {
                            return *dep.Spec.Replicas
                        }
                        return 1
                    }(),
                    dep.Status.ReadyReplicas,
                    dep.Status.UpdatedReplicas,
                    dep.Status.AvailableReplicas,
                    dep.Status.UnavailableReplicas,
                )
            }

            dep, err := k8s.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
            if err != nil {
                return fmt.Errorf("failed to get deployment %s/%s: %w", namespace, name, err)
            }

            if IsDeploymentReady(dep) {
                return nil
            }
        }
    }
}

func CreateDeployment(ctx context.Context, k8s kubernetes.Interface, deployment *appsv1.Deployment, timeout time.Duration) (*appsv1.Deployment, error) {
    created, err := k8s.AppsV1().Deployments(deployment.Namespace).Create(ctx, deployment, metav1.CreateOptions{})
    if err != nil {
        return nil, fmt.Errorf("failed to create deployment %s/%s: %w", deployment.Namespace, deployment.Name, err)
    }

    if err := WaitForDeploymentReady(ctx, k8s, created.Namespace, created.Name, timeout); err != nil {
        return nil, err
    }

    return created, nil
}

func UpdateDeployment(ctx context.Context, k8s kubernetes.Interface, deployment *appsv1.Deployment, timeout time.Duration) (*appsv1.Deployment, error) {
    updated, err := k8s.AppsV1().Deployments(deployment.Namespace).Update(ctx, deployment, metav1.UpdateOptions{})
    if err != nil {
        return nil, fmt.Errorf("failed to update deployment %s/%s: %w", deployment.Namespace, deployment.Name, err)
    }

    if err := WaitForDeploymentReady(ctx, k8s, updated.Namespace, updated.Name, timeout); err != nil {
        return nil, err
    }

    return updated, nil
}
