package activity

import (
	"context"
	"controller/utils"
	"fmt"
	"time"

	"launchs/shared/database"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const defaultDeploymentTimeout = 10 * time.Minute

// DeploymentActivity は Kubernetes Deployment の作成・更新・削除を担当します。
type DeploymentActivity struct{}
// ---- Activity メソッド --------------------------------------------------

// DeploymentApply は Deployment を作成または更新し、全 Pod が Ready になるまで待機します。
func (a *DeploymentActivity) DeploymentApply(ctx context.Context, spec DeploymentSpec) error {
	k8s := database.K8sClientset
	replicas := int32(spec.Replicas)
	if replicas <= 0 {
		replicas = 1
	}

	envVars := make([]corev1.EnvVar, 0, len(spec.EnvVars))
	for _, e := range spec.EnvVars {
		envVars = append(envVars, corev1.EnvVar{Name: e.Key, Value: e.Value})
	}

	containerPorts := make([]corev1.ContainerPort, 0, len(spec.Ports))
	for _, p := range spec.Ports {
		proto := corev1.ProtocolTCP
		if p.Protocol == "UDP" {
			proto = corev1.ProtocolUDP
		}
		containerPorts = append(containerPorts, corev1.ContainerPort{
			ContainerPort: int32(p.Port),
			Protocol:      proto,
		})
	}

	volumeMounts := make([]corev1.VolumeMount, 0, len(spec.VolumeMounts))
	volumes := make([]corev1.Volume, 0, len(spec.VolumeMounts))
	for _, vm := range spec.VolumeMounts {
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      vm.PVCName,
			MountPath: vm.MountPath,
		})
		volumes = append(volumes, corev1.Volume{
			Name: vm.PVCName,
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: vm.PVCName},
			},
		})
	}

	cpuReq := orDefault(spec.CPURequest, "100m")
	cpuLim := orDefault(spec.CPULimit, "500m")
	memReq := orDefault(spec.MemoryRequest, "128Mi")
	memLim := orDefault(spec.MemoryLimit, "512Mi")

	labels := spec.Labels
	if labels == nil {
		labels = map[string]string{}
	}
	labels["launchs-managed"] = "true"

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      spec.Name,
			Namespace: spec.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{MatchLabels: labels},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:            spec.Name,
							Image:           spec.Image,
							Env:             envVars,
							Ports:           containerPorts,
							VolumeMounts:    volumeMounts,
							ImagePullPolicy: corev1.PullAlways,
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse(cpuReq),
									corev1.ResourceMemory: resource.MustParse(memReq),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse(cpuLim),
									corev1.ResourceMemory: resource.MustParse(memLim),
								},
							},
						},
					},
					Volumes: volumes,
				},
			},
		},
	}

	existing, err := k8s.AppsV1().Deployments(spec.Namespace).Get(ctx, spec.Name, metav1.GetOptions{})
	if err != nil {
		if !k8serrors.IsNotFound(err) {
			return fmt.Errorf("Deployment 取得エラー: %w", err)
		}
		_, err = utils.CreateDeployment(ctx, k8s, deployment, defaultDeploymentTimeout)
		return err
	}

	deployment.ResourceVersion = existing.ResourceVersion

	// Deployment を更新
	_, err = utils.UpdateDeployment(ctx, k8s, deployment, defaultDeploymentTimeout)
	return err
}

// DeploymentDelete は Deployment を削除します。
func (a *DeploymentActivity) DeploymentDelete(ctx context.Context, namespace, name string) error {
	k8s := database.K8sClientset
	err := k8s.AppsV1().Deployments(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil && !k8serrors.IsNotFound(err) {
		return fmt.Errorf("Deployment 削除エラー %s/%s: %w", namespace, name, err)
	}
	return nil
}

// DeploymentUpdateReplicas は Deployment のレプリカ数を変更し、Pod 数の増減が完了するまで待機します。
func (a *DeploymentActivity) DeploymentUpdateReplicas(ctx context.Context, namespace, name string, replicas int) error {
	k8s := database.K8sClientset
	scale, err := k8s.AppsV1().Deployments(namespace).GetScale(ctx, name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("Deployment スケール取得エラー: %w", err)
	}
	scale.Spec.Replicas = int32(replicas)
	_, err = k8s.AppsV1().Deployments(namespace).UpdateScale(ctx, name, scale, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("Deployment スケール更新エラー: %w", err)
	}
	return utils.WaitForDeploymentReady(ctx, k8s, namespace, name, defaultDeploymentTimeout)
}

// DeploymentRolloutRestart は Deployment の rollout restart を行い、全 Pod が Ready になるまで待機します。
func (a *DeploymentActivity) DeploymentRolloutRestart(ctx context.Context, namespace, name string) error {
	k8s := database.K8sClientset
	deployment, err := k8s.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("Deployment 取得エラー: %w", err)
	}

	if deployment.Spec.Template.Annotations == nil {
		deployment.Spec.Template.Annotations = map[string]string{}
	}
	deployment.Spec.Template.Annotations["kubectl.kubernetes.io/restartedAt"] = metav1.Now().UTC().Format("2006-01-02T15:04:05Z")

	_, err = utils.UpdateDeployment(ctx, k8s, deployment, defaultDeploymentTimeout)
	
	return err
}

func orDefault(val, def string) string {
	if val == "" {
		return def
	}
	return val
}
