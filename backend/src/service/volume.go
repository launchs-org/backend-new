package service

import (
	"context"
	"fmt"
	"time"

	apperrors "launchs/shared/errors"
	"launchs/shared/model"
	"backend/repository"
	"launchs/shared/temporal"

	"github.com/google/uuid"
	"go.temporal.io/sdk/client"
)

// pvcResourceName は volumeID から PVC の k8s リソース名を生成します。
func pvcResourceName(name string, volumeID uuid.UUID) string {
	return fmt.Sprintf("%s-%s", name, volumeID.String())
}

type volumeService struct {
	projectRepo   repository.ProjectRepository
	containerRepo repository.ContainerRepository
	volumeRepo    repository.VolumeRepository
	temporal      client.Client
}

// NewVolumeService は VolumeService の実装を返します。
func NewVolumeService(
	projectRepo repository.ProjectRepository,
	containerRepo repository.ContainerRepository,
	volumeRepo repository.VolumeRepository,
	temporalClient client.Client,
) VolumeService {
	return &volumeService{
		projectRepo:   projectRepo,
		containerRepo: containerRepo,
		volumeRepo:    volumeRepo,
		temporal:      temporalClient,
	}
}

func (s *volumeService) List(ctx context.Context, userID string, projectID uuid.UUID) ([]model.Volume, error) {
	project, err := s.projectRepo.FindByID(ctx, projectID)
	if err != nil {
		return nil, &apperrors.NotFoundError{Resource: "project", ID: projectID.String()}
	}
	if project.UserID != userID {
		return nil, &apperrors.ForbiddenError{Message: "access denied"}
	}
	return s.volumeRepo.FindByProjectID(ctx, projectID)
}

func (s *volumeService) Create(ctx context.Context, userID string, projectID uuid.UUID, name string, sizeMB int, storageClass string) (string, string, error) {
	project, err := s.projectRepo.FindByID(ctx, projectID)
	if err != nil {
		return "", "", &apperrors.NotFoundError{Resource: "project", ID: projectID.String()}
	}
	if project.UserID != userID {
		return "", "", &apperrors.ForbiddenError{Message: "access denied"}
	}

	volumeID := uuid.New()
	if storageClass == "" {
		storageClass = "standard"
	}

	volume := &model.Volume{
		ID:           volumeID,
		ProjectID:    projectID,
		Name:         name,
		SizeMB:       sizeMB,
		StorageClass: storageClass,
		Status:       string(model.VolumeStatusPending),
	}
	if err := s.volumeRepo.Create(ctx, volume); err != nil {
		return "", "", fmt.Errorf("failed to create volume: %w", err)
	}

	pvcName := pvcResourceName(name, volumeID)
	storageSizeStr := fmt.Sprintf("%dMi", sizeMB)

	type createVolumeInput struct {
		VolumeID    uuid.UUID `json:"VolumeID"`
		Namespace   string    `json:"Namespace"`
		PVCName     string    `json:"PVCName"`
		StorageSize string    `json:"StorageSize"`
	}

	wfOpts := client.StartWorkflowOptions{
		ID:        fmt.Sprintf("create-volume-%s", volumeID.String()),
		TaskQueue: temporal.ControllerQueue,
	}
	we, err := s.temporal.ExecuteWorkflow(ctx, wfOpts, temporal.WorkflowCreateVolume, createVolumeInput{
		VolumeID:    volumeID,
		Namespace:   project.Namespace,
		PVCName:     pvcName,
		StorageSize: storageSizeStr,
	})
	if err != nil {
		return "", "", fmt.Errorf("failed to start CreateVolumeWorkflow: %w", err)
	}

	return volumeID.String(), we.GetID(), nil
}

func (s *volumeService) Delete(ctx context.Context, userID string, projectID, volumeID uuid.UUID) (string, error) {
	project, err := s.projectRepo.FindByID(ctx, projectID)
	if err != nil {
		return "", &apperrors.NotFoundError{Resource: "project", ID: projectID.String()}
	}
	if project.UserID != userID {
		return "", &apperrors.ForbiddenError{Message: "access denied"}
	}

	volume, err := s.volumeRepo.FindByID(ctx, volumeID)
	if err != nil {
		return "", &apperrors.NotFoundError{Resource: "volume", ID: volumeID.String()}
	}

	if err := s.volumeRepo.Delete(ctx, volumeID); err != nil {
		return "", fmt.Errorf("failed to delete volume: %w", err)
	}

	type deleteVolumeInput struct {
		VolumeID  uuid.UUID `json:"VolumeID"`
		Namespace string    `json:"Namespace"`
		PVCName   string    `json:"PVCName"`
	}

	wfOpts := client.StartWorkflowOptions{
		ID:        fmt.Sprintf("delete-volume-%s-%d", volumeID.String(), time.Now().UnixNano()),
		TaskQueue: temporal.ControllerQueue,
	}
	we, err := s.temporal.ExecuteWorkflow(ctx, wfOpts, temporal.WorkflowDeleteVolume, deleteVolumeInput{
		VolumeID:  volumeID,
		Namespace: project.Namespace,
		PVCName:   pvcResourceName(volume.Name, volumeID),
	})
	if err != nil {
		return "", fmt.Errorf("failed to start DeleteVolumeWorkflow: %w", err)
	}

	return we.GetID(), nil
}

func (s *volumeService) Mount(ctx context.Context, userID string, projectID, containerID, volumeID uuid.UUID, mountPath string) (string, error) {
	project, err := s.projectRepo.FindByID(ctx, projectID)
	if err != nil {
		return "", &apperrors.NotFoundError{Resource: "project", ID: projectID.String()}
	}
	if project.UserID != userID {
		return "", &apperrors.ForbiddenError{Message: "access denied"}
	}

	mount := &model.VolumeMount{
		ID:          uuid.New(),
		VolumeID:    volumeID,
		ContainerID: containerID,
		MountPath:   mountPath,
	}
	if err := s.volumeRepo.CreateMount(ctx, mount); err != nil {
		return "", fmt.Errorf("failed to create mount: %w", err)
	}

	volume, err := s.volumeRepo.FindByID(ctx, volumeID)
	if err != nil {
		return "", &apperrors.NotFoundError{Resource: "volume", ID: volumeID.String()}
	}

	type mountVolumeInput struct {
		ContainerID uuid.UUID `json:"ContainerID"`
		Namespace   string    `json:"Namespace"`
		VolumeID    uuid.UUID `json:"VolumeID"`
		PVCName     string    `json:"PVCName"`
		MountPath   string    `json:"MountPath"`
	}

	wfOpts := client.StartWorkflowOptions{
		ID:        fmt.Sprintf("mount-volume-%s-%s-%d", containerID.String(), volumeID.String(), time.Now().UnixNano()),
		TaskQueue: temporal.ControllerQueue,
	}
	we, err := s.temporal.ExecuteWorkflow(ctx, wfOpts, temporal.WorkflowMountVolume, mountVolumeInput{
		ContainerID: containerID,
		Namespace:   project.Namespace,
		VolumeID:    volumeID,
		PVCName:     pvcResourceName(volume.Name, volumeID),
		MountPath:   mountPath,
	})
	if err != nil {
		return "", fmt.Errorf("failed to start MountVolumeWorkflow: %w", err)
	}

	return we.GetID(), nil
}

func (s *volumeService) Unmount(ctx context.Context, userID string, projectID, containerID, volumeID uuid.UUID) (string, error) {
	project, err := s.projectRepo.FindByID(ctx, projectID)
	if err != nil {
		return "", &apperrors.NotFoundError{Resource: "project", ID: projectID.String()}
	}
	if project.UserID != userID {
		return "", &apperrors.ForbiddenError{Message: "access denied"}
	}

	if err := s.volumeRepo.DeleteMount(ctx, volumeID, containerID); err != nil {
		return "", fmt.Errorf("failed to delete mount: %w", err)
	}

	volume, err := s.volumeRepo.FindByID(ctx, volumeID)
	if err != nil {
		return "", &apperrors.NotFoundError{Resource: "volume", ID: volumeID.String()}
	}

	type unmountVolumeInput struct {
		ContainerID uuid.UUID `json:"ContainerID"`
		Namespace   string    `json:"Namespace"`
		VolumeID    uuid.UUID `json:"VolumeID"`
		PVCName     string    `json:"PVCName"`
	}

	wfOpts := client.StartWorkflowOptions{
		ID:        fmt.Sprintf("unmount-volume-%s-%s-%d", containerID.String(), volumeID.String(), time.Now().UnixNano()),
		TaskQueue: temporal.ControllerQueue,
	}
	we, err := s.temporal.ExecuteWorkflow(ctx, wfOpts, temporal.WorkflowUnmountVolume, unmountVolumeInput{
		ContainerID: containerID,
		Namespace:   project.Namespace,
		VolumeID:    volumeID,
		PVCName:     pvcResourceName(volume.Name, volumeID),
	})
	if err != nil {
		return "", fmt.Errorf("failed to start UnmountVolumeWorkflow: %w", err)
	}

	return we.GetID(), nil
}
