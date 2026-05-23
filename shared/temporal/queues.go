// Package temporal はコンポーネント間で共有する Temporal キュー名・ワークフロー名定数を定義します。
package temporal

// タスクキュー名。各コンポーネント（Worker）が対応するキューを listen します。
const (
	ControllerQueue = "controller-queue"
	WatcherQueue    = "watcher-queue"
	BuilderQueue    = "builder-queue"
)

// ワークフロー名定数。Backend が ExecuteWorkflow を呼ぶ際に使用します。
const (
	WorkflowCreateProject   = "CreateProjectWorkflow"
	WorkflowDeleteProject   = "DeleteProjectWorkflow"
	WorkflowCreateVolume    = "CreateVolumeWorkflow"
	WorkflowDeleteVolume    = "DeleteVolumeWorkflow"
	WorkflowBuildDeploy     = "BuildDeployWorkflow"
	WorkflowDeploy          = "DeployWorkflow"
	WorkflowRedeploy        = "RedeployWorkflow"
	WorkflowDeployProject   = "DeployProjectWorkflow"
	WorkflowScale           = "ScaleWorkflow"
	WorkflowDeleteContainer = "DeleteContainerWorkflow"
	WorkflowCreateService   = "CreateServiceWorkflow"
	WorkflowDeleteService   = "DeleteServiceWorkflow"
	WorkflowCreateIngress   = "CreateIngressWorkflow"
	WorkflowDeleteIngress   = "DeleteIngressWorkflow"
	WorkflowMountVolume     = "MountVolumeWorkflow"
	WorkflowUnmountVolume   = "UnmountVolumeWorkflow"
	WorkflowRestoreSnapshot = "RestoreSnapshotWorkflow"
	WorkflowBuild           = "BuildWorkflow"
	WorkflowDeployTemplate  = "DeployTemplateWorkflow"
)
