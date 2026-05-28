package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	apperrors "launchs/shared/errors"
	"launchs/shared/model"
	"backend/repository"
	"launchs/shared/temporal"

	"github.com/google/uuid"
	"go.temporal.io/sdk/client"
	"gopkg.in/yaml.v3"
)

// ---- log service ----

type logService struct {
	projectRepo   repository.ProjectRepository
	containerRepo repository.ContainerRepository
	buildJobRepo  repository.BuildJobRepository
	logRepo       repository.LogRepository
}

func NewLogService(
	projectRepo repository.ProjectRepository,
	containerRepo repository.ContainerRepository,
	buildJobRepo repository.BuildJobRepository,
	logRepo repository.LogRepository,
) LogService {
	return &logService{
		projectRepo:   projectRepo,
		containerRepo: containerRepo,
		buildJobRepo:  buildJobRepo,
		logRepo:       logRepo,
	}
}

func (s *logService) GetContainerLogs(ctx context.Context, userID string, projectID, containerID uuid.UUID, cursor time.Time, limit int, podName string) ([]model.ContainerLog, *time.Time, error) {
	if err := s.checkAccess(ctx, userID, projectID, containerID); err != nil {
		return nil, nil, err
	}

	logs, err := s.logRepo.FindAfterCursor(ctx, containerID, cursor, limit, podName)
	if err != nil {
		return nil, nil, err
	}

	var nextCursor *time.Time
	if len(logs) > 0 {
		t := logs[len(logs)-1].Timestamp
		nextCursor = &t
	}
	return logs, nextCursor, nil
}

func (s *logService) GetBuildJobLogs(ctx context.Context, userID string, projectID, buildJobID uuid.UUID, cursor time.Time, limit int) ([]model.ContainerLog, *time.Time, error) {
	project, err := s.projectRepo.FindByID(ctx, projectID)
	if err != nil {
		return nil, nil, &apperrors.NotFoundError{Resource: "project", ID: projectID.String()}
	}
	if project.UserID != userID {
		return nil, nil, &apperrors.ForbiddenError{Message: "access denied"}
	}

	logs, err := s.logRepo.FindBuildJobLogs(ctx, buildJobID, cursor, limit)
	if err != nil {
		return nil, nil, err
	}

	var nextCursor *time.Time
	if len(logs) > 0 {
		t := logs[len(logs)-1].Timestamp
		nextCursor = &t
	}
	return logs, nextCursor, nil
}

func (s *logService) checkAccess(ctx context.Context, userID string, projectID, containerID uuid.UUID) error {
	project, err := s.projectRepo.FindByID(ctx, projectID)
	if err != nil {
		return &apperrors.NotFoundError{Resource: "project", ID: projectID.String()}
	}
	if project.UserID != userID {
		return &apperrors.ForbiddenError{Message: "access denied"}
	}
	container, err := s.containerRepo.FindByID(ctx, containerID)
	if err != nil {
		return &apperrors.NotFoundError{Resource: "container", ID: containerID.String()}
	}
	if container.ProjectID != projectID {
		return &apperrors.ForbiddenError{Message: "access denied"}
	}
	return nil
}

// ---- metric service ----

type metricService struct {
	projectRepo   repository.ProjectRepository
	containerRepo repository.ContainerRepository
	metricRepo    repository.MetricRepository
}

func NewMetricService(
	projectRepo repository.ProjectRepository,
	containerRepo repository.ContainerRepository,
	metricRepo repository.MetricRepository,
) MetricService {
	return &metricService{
		projectRepo:   projectRepo,
		containerRepo: containerRepo,
		metricRepo:    metricRepo,
	}
}

func (s *metricService) Get(ctx context.Context, userID string, projectID, containerID uuid.UUID, from, to time.Time) ([]model.ContainerMetric, []model.ContainerMetric, error) {
	project, err := s.projectRepo.FindByID(ctx, projectID)
	if err != nil {
		return nil, nil, &apperrors.NotFoundError{Resource: "project", ID: projectID.String()}
	}
	if project.UserID != userID {
		return nil, nil, &apperrors.ForbiddenError{Message: "access denied"}
	}

	metrics, err := s.metricRepo.FindByRange(ctx, containerID, from, to)
	if err != nil {
		return nil, nil, err
	}

	return metrics, metrics, nil
}

// ---- build job service ----

type buildJobService struct {
	projectRepo   repository.ProjectRepository
	containerRepo repository.ContainerRepository
	buildJobRepo  repository.BuildJobRepository
	temporal      client.Client
}

func NewBuildJobService(
	projectRepo repository.ProjectRepository,
	containerRepo repository.ContainerRepository,
	buildJobRepo repository.BuildJobRepository,
	temporalClient client.Client,
) BuildJobService {
	return &buildJobService{
		projectRepo:   projectRepo,
		containerRepo: containerRepo,
		buildJobRepo:  buildJobRepo,
		temporal:      temporalClient,
	}
}

func (s *buildJobService) List(ctx context.Context, userID string, projectID, containerID uuid.UUID) ([]model.BuildJob, error) {
	project, err := s.projectRepo.FindByID(ctx, projectID)
	if err != nil {
		return nil, &apperrors.NotFoundError{Resource: "project", ID: projectID.String()}
	}
	if project.UserID != userID {
		return nil, &apperrors.ForbiddenError{Message: "access denied"}
	}
	return s.buildJobRepo.FindByContainerID(ctx, containerID)
}

func (s *buildJobService) Cancel(ctx context.Context, userID string, projectID, buildJobID uuid.UUID) error {
	project, err := s.projectRepo.FindByID(ctx, projectID)
	if err != nil {
		return &apperrors.NotFoundError{Resource: "project", ID: projectID.String()}
	}
	if project.UserID != userID {
		return &apperrors.ForbiddenError{Message: "access denied"}
	}

	job, err := s.buildJobRepo.FindByID(ctx, buildJobID)
	if err != nil {
		return &apperrors.NotFoundError{Resource: "build_job", ID: buildJobID.String()}
	}

	if job.TemporalWorkflowID != nil {
		// Temporal ワークフローをキャンセルします
		if err := s.temporal.CancelWorkflow(ctx, *job.TemporalWorkflowID, ""); err != nil {
			return fmt.Errorf("failed to cancel workflow: %w", err)
		}
	}

	return s.buildJobRepo.UpdateStatus(ctx, buildJobID, string(model.BuildJobStatusFailed))
}

// ---- template service ----

type templateService struct {
	templateDir string
}

func NewTemplateService(templateDir string) TemplateService {
	return &templateService{templateDir: templateDir}
}

func (s *templateService) List(ctx context.Context) ([]TemplateSummary, error) {
	entries, err := os.ReadDir(s.templateDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read template dir: %w", err)
	}

	var summaries []TemplateSummary
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".yaml") {
			tf, err := s.loadFile(entry.Name())
			if err != nil {
				continue
			}
			summaries = append(summaries, TemplateSummary{
				Name:        tf.Name,
				DisplayName: tf.DisplayName,
				Category:    tf.Category,
				Description: tf.Description,
				Version:     tf.Version,
				Icon:        tf.Icon,
				Color:       tf.Color,
			})
		}
	}
	return summaries, nil
}

func (s *templateService) Get(ctx context.Context, name string) (*TemplateDetail, error) {
	tf, err := s.loadFile(name + ".yaml")
	if err != nil {
		return nil, &apperrors.NotFoundError{Resource: "template", ID: name}
	}

	return &TemplateDetail{
		TemplateSummary: TemplateSummary{
			Name:        tf.Name,
			DisplayName: tf.DisplayName,
			Category:    tf.Category,
			Description: tf.Description,
			Version:     tf.Version,
			Icon:        tf.Icon,
			Color:       tf.Color,
		},
		Image:   tf.Image,
		EnvVars: tf.EnvVars,
		Volume:  tf.Volume,
		Ports:   tf.Ports,
		Spec:    tf.Spec,
	}, nil
}

func (s *templateService) loadFile(filename string) (*TemplateFile, error) {
	data, err := os.ReadFile(filepath.Join(s.templateDir, filename))
	if err != nil {
		return nil, err
	}
	var tf TemplateFile
	if err := yaml.Unmarshal(data, &tf); err != nil {
		return nil, err
	}
	return &tf, nil
}

// ---- snapshot service ----

type snapshotService struct {
	projectRepo   repository.ProjectRepository
	containerRepo repository.ContainerRepository
	snapshotRepo  repository.SnapshotRepository
	temporal      client.Client
}

func NewSnapshotService(
	projectRepo repository.ProjectRepository,
	containerRepo repository.ContainerRepository,
	snapshotRepo repository.SnapshotRepository,
	temporalClient client.Client,
) SnapshotService {
	return &snapshotService{
		projectRepo:   projectRepo,
		containerRepo: containerRepo,
		snapshotRepo:  snapshotRepo,
		temporal:      temporalClient,
	}
}

func (s *snapshotService) List(ctx context.Context, userID string, projectID uuid.UUID) ([]model.Snapshot, error) {
	project, err := s.projectRepo.FindByID(ctx, projectID)
	if err != nil {
		return nil, &apperrors.NotFoundError{Resource: "project", ID: projectID.String()}
	}
	if project.UserID != userID {
		return nil, &apperrors.ForbiddenError{Message: "access denied"}
	}
	return s.snapshotRepo.FindByProjectID(ctx, projectID)
}

func (s *snapshotService) Get(ctx context.Context, userID string, projectID, snapshotID uuid.UUID) (*model.Snapshot, error) {
	project, err := s.projectRepo.FindByID(ctx, projectID)
	if err != nil {
		return nil, &apperrors.NotFoundError{Resource: "project", ID: projectID.String()}
	}
	if project.UserID != userID {
		return nil, &apperrors.ForbiddenError{Message: "access denied"}
	}

	snapshot, err := s.snapshotRepo.FindByID(ctx, snapshotID)
	if err != nil {
		return nil, &apperrors.NotFoundError{Resource: "snapshot", ID: snapshotID.String()}
	}
	return snapshot, nil
}

type RestoreSnapshotWorkflowInput struct {
	SnapshotID string `json:"snapshot_id"`
	ProjectID  string `json:"project_id"`
}

func (s *snapshotService) Restore(ctx context.Context, userID string, projectID, snapshotID uuid.UUID) (string, error) {
	project, err := s.projectRepo.FindByID(ctx, projectID)
	if err != nil {
		return "", &apperrors.NotFoundError{Resource: "project", ID: projectID.String()}
	}
	if project.UserID != userID {
		return "", &apperrors.ForbiddenError{Message: "access denied"}
	}

	wfOpts := client.StartWorkflowOptions{
		ID:        fmt.Sprintf("restore-snapshot-%s-%d", snapshotID.String(), time.Now().UnixNano()),
		TaskQueue: temporal.ControllerQueue,
	}
	we, err := s.temporal.ExecuteWorkflow(ctx, wfOpts, temporal.WorkflowRestoreSnapshot, RestoreSnapshotWorkflowInput{
		SnapshotID: snapshotID.String(),
		ProjectID:  projectID.String(),
	})
	if err != nil {
		return "", fmt.Errorf("failed to start RestoreSnapshotWorkflow: %w", err)
	}

	return we.GetID(), nil
}

// ---- connection service ----

type connectionService struct {
	projectRepo    repository.ProjectRepository
	connectionRepo repository.ServiceConnectionRepository
}

func NewConnectionService(
	projectRepo repository.ProjectRepository,
	connectionRepo repository.ServiceConnectionRepository,
) ConnectionService {
	return &connectionService{
		projectRepo:    projectRepo,
		connectionRepo: connectionRepo,
	}
}

func (s *connectionService) List(ctx context.Context, userID string, projectID uuid.UUID) ([]model.ServiceConnection, error) {
	project, err := s.projectRepo.FindByID(ctx, projectID)
	if err != nil {
		return nil, &apperrors.NotFoundError{Resource: "project", ID: projectID.String()}
	}
	if project.UserID != userID {
		return nil, &apperrors.ForbiddenError{Message: "access denied"}
	}
	return s.connectionRepo.FindByProjectID(ctx, projectID)
}
