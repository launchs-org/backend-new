package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.temporal.io/sdk/client"
	"launchs/shared/model"
	"launchs/shared/temporal"
	apperrors "launchs/shared/errors"
	"backend/repository"
)

// CreateVolumeWorkflowInput は CreateVolumeWorkflow の入力です。
type CreateVolumeWorkflowInput struct {
	VolumeID     string `json:"volume_id"`
	ProjectID    string `json:"project_id"`
	Namespace    string `json:"namespace"`
	Name         string `json:"name"`
	SizeMB       int    `json:"size_mb"`
	StorageClass string `json:"storage_class"`
}

// DeleteVolumeWorkflowInput は DeleteVolumeWorkflow の入力です。
type DeleteVolumeWorkflowInput struct {
	VolumeID  string `json:"volume_id"`
	ProjectID string `json:"project_id"`
	Namespace string `json:"namespace"`
	PVCName   string `json:"pvc_name"`
}

// MountVolumeWorkflowInput は MountVolumeWorkflow の入力です。
type MountVolumeWorkflowInput struct {
	VolumeID    string `json:"volume_id"`
	ContainerID string `json:"container_id"`
	ProjectID   string `json:"project_id"`
	Namespace   string `json:"namespace"`
	MountPath   string `json:"mount_path"`
}

// UnmountVolumeWorkflowInput は UnmountVolumeWorkflow の入力です。
type UnmountVolumeWorkflowInput struct {
	VolumeID    string `json:"volume_id"`
	ContainerID string `json:"container_id"`
	ProjectID   string `json:"project_id"`
	Namespace   string `json:"namespace"`
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

	wfOpts := client.StartWorkflowOptions{
		ID:        fmt.Sprintf("create-volume-%s", volumeID.String()),
		TaskQueue: temporal.ControllerQueue,
	}
	we, err := s.temporal.ExecuteWorkflow(ctx, wfOpts, temporal.WorkflowCreateVolume, CreateVolumeWorkflowInput{
		VolumeID:     volumeID.String(),
		ProjectID:    projectID.String(),
		Namespace:    project.Namespace,
		Name:         name,
		SizeMB:       sizeMB,
		StorageClass: storageClass,
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

	pvcName := fmt.Sprintf("%s-%s", volume.Name, volumeID.String()[:8])
	wfOpts := client.StartWorkflowOptions{
		ID:        fmt.Sprintf("delete-volume-%s-%d", volumeID.String(), time.Now().UnixNano()),
		TaskQueue: temporal.ControllerQueue,
	}
	we, err := s.temporal.ExecuteWorkflow(ctx, wfOpts, temporal.WorkflowDeleteVolume, DeleteVolumeWorkflowInput{
		VolumeID:  volumeID.String(),
		ProjectID: projectID.String(),
		Namespace: project.Namespace,
		PVCName:   pvcName,
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

	wfOpts := client.StartWorkflowOptions{
		ID:        fmt.Sprintf("mount-volume-%s-%s-%d", containerID.String(), volumeID.String(), time.Now().UnixNano()),
		TaskQueue: temporal.ControllerQueue,
	}
	we, err := s.temporal.ExecuteWorkflow(ctx, wfOpts, temporal.WorkflowMountVolume, MountVolumeWorkflowInput{
		VolumeID:    volumeID.String(),
		ContainerID: containerID.String(),
		ProjectID:   projectID.String(),
		Namespace:   project.Namespace,
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

	wfOpts := client.StartWorkflowOptions{
		ID:        fmt.Sprintf("unmount-volume-%s-%s-%d", containerID.String(), volumeID.String(), time.Now().UnixNano()),
		TaskQueue: temporal.ControllerQueue,
	}
	we, err := s.temporal.ExecuteWorkflow(ctx, wfOpts, temporal.WorkflowUnmountVolume, UnmountVolumeWorkflowInput{
		VolumeID:    volumeID.String(),
		ContainerID: containerID.String(),
		ProjectID:   projectID.String(),
		Namespace:   project.Namespace,
	})
	if err != nil {
		return "", fmt.Errorf("failed to start UnmountVolumeWorkflow: %w", err)
	}

	return we.GetID(), nil
}
