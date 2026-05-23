package activity

// DeploymentSpec は Kubernetes Deployment を作成・更新するための仕様です。
// ワークフローとアクティビティ間でシリアライズされるため、k8s 型は使いません。
type DeploymentSpec struct {
	Namespace    string
	Name         string // {container-name}-{container-id[0:8]}
	Image        string // Harbor フル参照
	Replicas     int
	CPURequest   string // "100m"
	CPULimit     string
	MemoryRequest string // "128Mi"
	MemoryLimit  string
	EnvVars      []EnvVar
	Ports        []Port
	VolumeMounts []VolumeMount
	Labels       map[string]string // launchs-managed=true, container-id={id}
}

// ServiceSpec は Kubernetes Service を作成・更新するための仕様です。
type ServiceSpec struct {
	Namespace   string
	Name        string
	Ports       []Port
	SelectorLabels map[string]string
}

// IngressSpec は Traefik IngressRoute を作成・更新するための仕様です。
type IngressSpec struct {
	Namespace   string
	Name        string
	ServiceName string
	Host        string // {container-name}-{project-uuid}.launchs.org
	Port        int
}

// PVCSpec は PersistentVolumeClaim を作成するための仕様です。
type PVCSpec struct {
	Namespace   string
	Name        string // {volume-name}-{volume-id[0:8]}
	StorageSize string // "1Gi"
}

// EnvVar はコンテナの環境変数です。
type EnvVar struct {
	Key   string
	Value string
}

// Port はコンテナのポート設定です。
type Port struct {
	Port     int
	Protocol string // TCP / UDP
}

// VolumeMount はボリュームマウント設定です。
type VolumeMount struct {
	PVCName   string // PVC のリソース名
	MountPath string
}

