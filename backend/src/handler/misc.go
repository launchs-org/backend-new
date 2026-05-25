package handler

import (
	"strconv"
	"time"

	"backend/response"
	"backend/service"
	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
)

// EnvVarHandler は環境変数関連のリクエストを処理します。
type EnvVarHandler struct {
	svc service.EnvVarService
}

func NewEnvVarHandler(svc service.EnvVarService) *EnvVarHandler {
	return &EnvVarHandler{svc: svc}
}

func (h *EnvVarHandler) ListProject(c *echo.Context) error {
	userID := c.Get("UserID").(string)
	projectID, err := uuid.Parse(c.Param("project_id"))
	if err != nil {
		return badRequest(c, "invalid project_id")
	}
	vars, err := h.svc.ListProject(c.Request().Context(), userID, projectID)
	if err != nil {
		return response.Error(c, err)
	}
	result := make([]map[string]interface{}, len(vars))
	for i, v := range vars {
		result[i] = map[string]interface{}{"id": v.ID.String(), "key": v.Key, "value": v.Value}
	}
	return response.OK(c, result)
}

func (h *EnvVarHandler) UpsertProject(c *echo.Context) error {
	userID := c.Get("UserID").(string)
	projectID, err := uuid.Parse(c.Param("project_id"))
	if err != nil {
		return badRequest(c, "invalid project_id")
	}

	var req struct {
		EnvVars []envVarInputJSON `json:"env_vars"`
	}
	if err := c.Bind(&req); err != nil {
		return badRequest(c, err.Error())
	}

	vars := make([]service.EnvVarInput, len(req.EnvVars))
	for i, v := range req.EnvVars {
		vars[i] = service.EnvVarInput{Key: v.Key, Value: v.Value}
	}

	if err := h.svc.UpsertProject(c.Request().Context(), userID, projectID, vars); err != nil {
		return response.Error(c, err)
	}
	return response.OK(c, map[string]interface{}{})
}

func (h *EnvVarHandler) DeleteProject(c *echo.Context) error {
	userID := c.Get("UserID").(string)
	projectID, err := uuid.Parse(c.Param("project_id"))
	if err != nil {
		return badRequest(c, "invalid project_id")
	}

	var req struct {
		Keys []string `json:"keys"`
	}
	if err := c.Bind(&req); err != nil {
		return badRequest(c, err.Error())
	}

	for _, key := range req.Keys {
		if err := h.svc.DeleteProject(c.Request().Context(), userID, projectID, key); err != nil {
			return response.Error(c, err)
		}
	}
	return response.OK(c, map[string]interface{}{})
}

func (h *EnvVarHandler) ListContainer(c *echo.Context) error {
	userID := c.Get("UserID").(string)
	projectID, _ := uuid.Parse(c.Param("project_id"))
	containerID, _ := uuid.Parse(c.Param("container_id"))

	vars, err := h.svc.ListContainer(c.Request().Context(), userID, projectID, containerID)
	if err != nil {
		return response.Error(c, err)
	}
	result := make([]map[string]interface{}, len(vars))
	for i, v := range vars {
		result[i] = map[string]interface{}{"id": v.ID.String(), "key": v.Key, "value": v.Value}
	}
	return response.OK(c, result)
}

func (h *EnvVarHandler) UpsertContainer(c *echo.Context) error {
	userID := c.Get("UserID").(string)
	projectID, _ := uuid.Parse(c.Param("project_id"))
	containerID, _ := uuid.Parse(c.Param("container_id"))

	var req struct {
		EnvVars []envVarInputJSON `json:"env_vars"`
	}
	if err := c.Bind(&req); err != nil {
		return badRequest(c, err.Error())
	}

	vars := make([]service.EnvVarInput, len(req.EnvVars))
	for i, v := range req.EnvVars {
		vars[i] = service.EnvVarInput{Key: v.Key, Value: v.Value}
	}

	if err := h.svc.UpsertContainer(c.Request().Context(), userID, projectID, containerID, vars); err != nil {
		return response.Error(c, err)
	}
	return response.OK(c, map[string]interface{}{})
}

func (h *EnvVarHandler) DeleteContainer(c *echo.Context) error {
	userID := c.Get("UserID").(string)
	projectID, _ := uuid.Parse(c.Param("project_id"))
	containerID, _ := uuid.Parse(c.Param("container_id"))

	var req struct {
		Keys []string `json:"keys"`
	}
	if err := c.Bind(&req); err != nil {
		return badRequest(c, err.Error())
	}

	for _, key := range req.Keys {
		if err := h.svc.DeleteContainer(c.Request().Context(), userID, projectID, containerID, key); err != nil {
			return response.Error(c, err)
		}
	}
	return response.OK(c, map[string]interface{}{})
}

func (h *EnvVarHandler) GetSelectedProjectEnvVarKeys(c *echo.Context) error {
	userID := c.Get("UserID").(string)
	projectID, _ := uuid.Parse(c.Param("project_id"))
	containerID, _ := uuid.Parse(c.Param("container_id"))

	keys, err := h.svc.GetSelectedProjectEnvVarKeys(c.Request().Context(), userID, projectID, containerID)
	if err != nil {
		return response.Error(c, err)
	}
	return response.OK(c, map[string]interface{}{"keys": keys})
}

func (h *EnvVarHandler) SetSelectedProjectEnvVarKeys(c *echo.Context) error {
	userID := c.Get("UserID").(string)
	projectID, _ := uuid.Parse(c.Param("project_id"))
	containerID, _ := uuid.Parse(c.Param("container_id"))

	var req struct {
		Keys []string `json:"keys"`
	}
	if err := c.Bind(&req); err != nil {
		return badRequest(c, err.Error())
	}
	if req.Keys == nil {
		req.Keys = []string{}
	}

	if err := h.svc.SetSelectedProjectEnvVarKeys(c.Request().Context(), userID, projectID, containerID, req.Keys); err != nil {
		return response.Error(c, err)
	}
	return response.OK(c, map[string]interface{}{})
}

// ---- PortHandler ----

type PortHandler struct {
	svc service.PortService
}

func NewPortHandler(svc service.PortService) *PortHandler {
	return &PortHandler{svc: svc}
}

func (h *PortHandler) List(c *echo.Context) error {
	userID := c.Get("UserID").(string)
	projectID, _ := uuid.Parse(c.Param("project_id"))
	containerID, _ := uuid.Parse(c.Param("container_id"))

	ports, err := h.svc.List(c.Request().Context(), userID, projectID, containerID)
	if err != nil {
		return response.Error(c, err)
	}
	result := make([]map[string]interface{}, len(ports))
	for i, p := range ports {
		result[i] = map[string]interface{}{
			"id":       p.ID.String(),
			"port":     p.Port,
			"protocol": p.Protocol,
		}
	}
	return response.OK(c, result)
}

func (h *PortHandler) Create(c *echo.Context) error {
	userID := c.Get("UserID").(string)
	projectID, _ := uuid.Parse(c.Param("project_id"))
	containerID, _ := uuid.Parse(c.Param("container_id"))

	var req struct {
		Port     int    `json:"port"`
		Protocol string `json:"protocol"`
	}
	if err := c.Bind(&req); err != nil {
		return badRequest(c, err.Error())
	}

	port, err := h.svc.Create(c.Request().Context(), userID, projectID, containerID, req.Port, req.Protocol)
	if err != nil {
		return response.Error(c, err)
	}
	return response.Created(c, map[string]interface{}{
		"id":       port.ID.String(),
		"port":     port.Port,
		"protocol": port.Protocol,
	})
}

func (h *PortHandler) Delete(c *echo.Context) error {
	userID := c.Get("UserID").(string)
	projectID, _ := uuid.Parse(c.Param("project_id"))
	containerID, _ := uuid.Parse(c.Param("container_id"))
	portID, _ := uuid.Parse(c.Param("port_id"))

	if err := h.svc.Delete(c.Request().Context(), userID, projectID, containerID, portID); err != nil {
		return response.Error(c, err)
	}
	return response.OK(c, map[string]interface{}{})
}

// ---- RouteHandler ----

type RouteHandler struct {
	svc service.RouteService
}

func NewRouteHandler(svc service.RouteService) *RouteHandler {
	return &RouteHandler{svc: svc}
}

func (h *RouteHandler) List(c *echo.Context) error {
	userID := c.Get("UserID").(string)
	projectID, _ := uuid.Parse(c.Param("project_id"))
	containerID, _ := uuid.Parse(c.Param("container_id"))

	routes, err := h.svc.List(c.Request().Context(), userID, projectID, containerID)
	if err != nil {
		return response.Error(c, err)
	}
	result := make([]map[string]interface{}, len(routes))
	for i, r := range routes {
		result[i] = map[string]interface{}{
			"id":         r.ID.String(),
			"type":       r.Type,
			"port":       r.Port,
			"protocol":   r.Protocol,
			"subdomain":  r.Subdomain,
			"created_at": r.CreatedAt,
		}
	}
	return response.OK(c, result)
}

func (h *RouteHandler) CreateService(c *echo.Context) error {
	userID := c.Get("UserID").(string)
	projectID, _ := uuid.Parse(c.Param("project_id"))
	containerID, _ := uuid.Parse(c.Param("container_id"))

	var req struct {
		Port     int    `json:"port"`
		Protocol string `json:"protocol"`
	}
	if err := c.Bind(&req); err != nil {
		return badRequest(c, err.Error())
	}
	if req.Protocol != "TCP" && req.Protocol != "UDP" {
		return badRequest(c, "protocol must be TCP or UDP")
	}

	workflowID, err := h.svc.CreateService(c.Request().Context(), userID, projectID, containerID, req.Port, req.Protocol)
	if err != nil {
		return response.Error(c, err)
	}
	return response.Created(c, map[string]string{"workflow_id": workflowID})
}

func (h *RouteHandler) CreateIngress(c *echo.Context) error {
	userID := c.Get("UserID").(string)
	projectID, _ := uuid.Parse(c.Param("project_id"))
	containerID, _ := uuid.Parse(c.Param("container_id"))

	var req struct {
		Port int `json:"port"`
	}
	if err := c.Bind(&req); err != nil {
		return badRequest(c, err.Error())
	}

	routeID, subdomain, workflowID, err := h.svc.CreateIngress(c.Request().Context(), userID, projectID, containerID, req.Port)
	if err != nil {
		return response.Error(c, err)
	}
	return response.Created(c, map[string]string{
		"id":          routeID,
		"subdomain":   subdomain,
		"workflow_id": workflowID,
	})
}

func (h *RouteHandler) Delete(c *echo.Context) error {
	userID := c.Get("UserID").(string)
	projectID, _ := uuid.Parse(c.Param("project_id"))
	containerID, _ := uuid.Parse(c.Param("container_id"))
	routeID, _ := uuid.Parse(c.Param("route_id"))

	workflowID, err := h.svc.Delete(c.Request().Context(), userID, projectID, containerID, routeID)
	if err != nil {
		return response.Error(c, err)
	}
	return response.OK(c, map[string]string{"workflow_id": workflowID})
}

// ---- LogHandler ----

type LogHandler struct {
	svc service.LogService
}

func NewLogHandler(svc service.LogService) *LogHandler {
	return &LogHandler{svc: svc}
}

func (h *LogHandler) GetContainer(c *echo.Context) error {
	userID := c.Get("UserID").(string)
	projectID, _ := uuid.Parse(c.Param("project_id"))
	containerID, _ := uuid.Parse(c.Param("container_id"))

	// カーソルが未指定の場合は Unix エポックから取得します
	cursor := time.Unix(0, 0)
	if cursorStr := c.QueryParam("cursor"); cursorStr != "" {
		if t, err := time.Parse(time.RFC3339Nano, cursorStr); err == nil {
			cursor = t
		}
	}

	limit := 100
	if limitStr := c.QueryParam("limit"); limitStr != "" {
		if n, err := strconv.Atoi(limitStr); err == nil {
			limit = n
		}
	}

	podName := c.QueryParam("pod_name")

	logs, nextCursor, err := h.svc.GetContainerLogs(c.Request().Context(), userID, projectID, containerID, cursor, limit, podName)
	if err != nil {
		return response.Error(c, err)
	}

	return response.OK(c, map[string]interface{}{
		"logs":        logs,
		"next_cursor": nextCursor,
	})
}

func (h *LogHandler) GetBuildJob(c *echo.Context) error {
	userID := c.Get("UserID").(string)
	projectID, _ := uuid.Parse(c.Param("project_id"))
	buildJobID, _ := uuid.Parse(c.Param("build_job_id"))

	cursor := time.Unix(0, 0)
	if cursorStr := c.QueryParam("cursor"); cursorStr != "" {
		if t, err := time.Parse(time.RFC3339Nano, cursorStr); err == nil {
			cursor = t
		}
	}

	limit := 100
	if limitStr := c.QueryParam("limit"); limitStr != "" {
		if n, err := strconv.Atoi(limitStr); err == nil {
			limit = n
		}
	}

	logs, nextCursor, err := h.svc.GetBuildJobLogs(c.Request().Context(), userID, projectID, buildJobID, cursor, limit)
	if err != nil {
		return response.Error(c, err)
	}

	return response.OK(c, map[string]interface{}{
		"logs":        logs,
		"next_cursor": nextCursor,
	})
}

// ---- MetricHandler ----

type MetricHandler struct {
	svc service.MetricService
}

func NewMetricHandler(svc service.MetricService) *MetricHandler {
	return &MetricHandler{svc: svc}
}

func (h *MetricHandler) Get(c *echo.Context) error {
	userID := c.Get("UserID").(string)
	projectID, _ := uuid.Parse(c.Param("project_id"))
	containerID, _ := uuid.Parse(c.Param("container_id"))

	var from, to time.Time
	if s := c.QueryParam("from"); s != "" {
		from, _ = time.Parse(time.RFC3339, s)
	}
	if s := c.QueryParam("to"); s != "" {
		to, _ = time.Parse(time.RFC3339, s)
	}

	cpu, memory, err := h.svc.Get(c.Request().Context(), userID, projectID, containerID, from, to)
	if err != nil {
		return response.Error(c, err)
	}

	type metricPoint struct {
		Timestamp time.Time `json:"timestamp"`
		Value     float64   `json:"value"`
	}

	cpuPoints := make([]metricPoint, len(cpu))
	for i, m := range cpu {
		cpuPoints[i] = metricPoint{Timestamp: m.Timestamp, Value: m.CPUUsage}
	}
	memPoints := make([]metricPoint, len(memory))
	for i, m := range memory {
		memPoints[i] = metricPoint{Timestamp: m.Timestamp, Value: float64(m.MemoryBytes)}
	}

	return response.OK(c, map[string]interface{}{
		"cpu":    cpuPoints,
		"memory": memPoints,
	})
}

// ---- BuildJobHandler ----

type BuildJobHandler struct {
	svc service.BuildJobService
}

func NewBuildJobHandler(svc service.BuildJobService) *BuildJobHandler {
	return &BuildJobHandler{svc: svc}
}

func (h *BuildJobHandler) List(c *echo.Context) error {
	userID := c.Get("UserID").(string)
	projectID, _ := uuid.Parse(c.Param("project_id"))
	containerID, _ := uuid.Parse(c.Param("container_id"))

	jobs, err := h.svc.List(c.Request().Context(), userID, projectID, containerID)
	if err != nil {
		return response.Error(c, err)
	}
	result := make([]map[string]interface{}, len(jobs))
	for i, j := range jobs {
		result[i] = map[string]interface{}{
			"id":                   j.ID.String(),
			"git_repo":             j.GitRepo,
			"git_branch":           j.GitBranch,
			"git_commit":           j.GitCommit,
			"status":               j.Status,
			"temporal_workflow_id": j.TemporalWorkflowID,
			"started_at":           j.StartedAt,
			"finished_at":          j.FinishedAt,
			"image_id":             j.ImageID,
			"created_at":           j.CreatedAt,
		}
	}
	return response.OK(c, result)
}

func (h *BuildJobHandler) Cancel(c *echo.Context) error {
	userID := c.Get("UserID").(string)
	projectID, _ := uuid.Parse(c.Param("project_id"))
	buildJobID, _ := uuid.Parse(c.Param("build_job_id"))

	if err := h.svc.Cancel(c.Request().Context(), userID, projectID, buildJobID); err != nil {
		return response.Error(c, err)
	}
	return response.OK(c, map[string]interface{}{})
}

// ---- TemplateHandler ----

type TemplateHandler struct {
	svc service.TemplateService
}

func NewTemplateHandler(svc service.TemplateService) *TemplateHandler {
	return &TemplateHandler{svc: svc}
}

func (h *TemplateHandler) List(c *echo.Context) error {
	templates, err := h.svc.List(c.Request().Context())
	if err != nil {
		return response.Error(c, err)
	}
	return response.OK(c, templates)
}

func (h *TemplateHandler) Get(c *echo.Context) error {
	name := c.Param("template_name")
	tmpl, err := h.svc.Get(c.Request().Context(), name)
	if err != nil {
		return response.Error(c, err)
	}
	return response.OK(c, tmpl)
}

// ---- SnapshotHandler ----

type SnapshotHandler struct {
	svc service.SnapshotService
}

func NewSnapshotHandler(svc service.SnapshotService) *SnapshotHandler {
	return &SnapshotHandler{svc: svc}
}

func (h *SnapshotHandler) List(c *echo.Context) error {
	userID := c.Get("UserID").(string)
	projectID, _ := uuid.Parse(c.Param("project_id"))

	snapshots, err := h.svc.List(c.Request().Context(), userID, projectID)
	if err != nil {
		return response.Error(c, err)
	}
	result := make([]map[string]interface{}, len(snapshots))
	for i, s := range snapshots {
		result[i] = map[string]interface{}{
			"id":          s.ID.String(),
			"description": s.Description,
			"created_at":  s.CreatedAt,
		}
	}
	return response.OK(c, result)
}

func (h *SnapshotHandler) Get(c *echo.Context) error {
	userID := c.Get("UserID").(string)
	projectID, _ := uuid.Parse(c.Param("project_id"))
	snapshotID, _ := uuid.Parse(c.Param("snapshot_id"))

	snapshot, err := h.svc.Get(c.Request().Context(), userID, projectID, snapshotID)
	if err != nil {
		return response.Error(c, err)
	}
	return response.OK(c, map[string]interface{}{
		"id":          snapshot.ID.String(),
		"description": snapshot.Description,
		"created_at":  snapshot.CreatedAt,
	})
}

func (h *SnapshotHandler) Restore(c *echo.Context) error {
	userID := c.Get("UserID").(string)
	projectID, _ := uuid.Parse(c.Param("project_id"))
	snapshotID, _ := uuid.Parse(c.Param("snapshot_id"))

	workflowID, err := h.svc.Restore(c.Request().Context(), userID, projectID, snapshotID)
	if err != nil {
		return response.Error(c, err)
	}
	return response.OK(c, map[string]string{"workflow_id": workflowID})
}

// ---- VolumeHandler ----

type VolumeHandler struct {
	svc service.VolumeService
}

func NewVolumeHandler(svc service.VolumeService) *VolumeHandler {
	return &VolumeHandler{svc: svc}
}

func (h *VolumeHandler) List(c *echo.Context) error {
	userID := c.Get("UserID").(string)
	projectID, _ := uuid.Parse(c.Param("project_id"))

	volumes, err := h.svc.List(c.Request().Context(), userID, projectID)
	if err != nil {
		return response.Error(c, err)
	}
	result := make([]map[string]interface{}, len(volumes))
	for i, v := range volumes {
		mounts := make([]map[string]interface{}, len(v.Mounts))
		for j, m := range v.Mounts {
			mounts[j] = map[string]interface{}{
				"id":           m.ID.String(),
				"volume_id":    m.VolumeID.String(),
				"container_id": m.ContainerID.String(),
				"mount_path":   m.MountPath,
			}
		}
		result[i] = map[string]interface{}{
			"id":            v.ID.String(),
			"name":          v.Name,
			"size_mb":       v.SizeMB,
			"storage_class": v.StorageClass,
			"status":        v.Status,
			"mounts":        mounts,
			"created_at":    v.CreatedAt,
		}
	}
	return response.OK(c, result)
}

func (h *VolumeHandler) Create(c *echo.Context) error {
	userID := c.Get("UserID").(string)
	projectID, _ := uuid.Parse(c.Param("project_id"))

	var req struct {
		Name         string `json:"name"`
		SizeMB       int    `json:"size_mb"`
		StorageClass string `json:"storage_class"`
	}
	if err := c.Bind(&req); err != nil {
		return badRequest(c, err.Error())
	}

	volumeID, workflowID, err := h.svc.Create(c.Request().Context(), userID, projectID, req.Name, req.SizeMB, req.StorageClass)
	if err != nil {
		return response.Error(c, err)
	}
	return response.Created(c, map[string]string{
		"volume_id":   volumeID,
		"workflow_id": workflowID,
	})
}

func (h *VolumeHandler) Delete(c *echo.Context) error {
	userID := c.Get("UserID").(string)
	projectID, _ := uuid.Parse(c.Param("project_id"))
	volumeID, _ := uuid.Parse(c.Param("volume_id"))

	workflowID, err := h.svc.Delete(c.Request().Context(), userID, projectID, volumeID)
	if err != nil {
		return response.Error(c, err)
	}
	return response.OK(c, map[string]string{"workflow_id": workflowID})
}

func (h *VolumeHandler) Mount(c *echo.Context) error {
	userID := c.Get("UserID").(string)
	projectID, _ := uuid.Parse(c.Param("project_id"))
	containerID, _ := uuid.Parse(c.Param("container_id"))

	var req struct {
		VolumeID  string `json:"volume_id"`
		MountPath string `json:"mount_path"`
	}
	if err := c.Bind(&req); err != nil {
		return badRequest(c, err.Error())
	}

	volumeID, err := uuid.Parse(req.VolumeID)
	if err != nil {
		return badRequest(c, "invalid volume_id")
	}

	workflowID, err := h.svc.Mount(c.Request().Context(), userID, projectID, containerID, volumeID, req.MountPath)
	if err != nil {
		return response.Error(c, err)
	}
	return response.OK(c, map[string]string{"workflow_id": workflowID})
}

func (h *VolumeHandler) Unmount(c *echo.Context) error {
	userID := c.Get("UserID").(string)
	projectID, _ := uuid.Parse(c.Param("project_id"))
	containerID, _ := uuid.Parse(c.Param("container_id"))
	volumeID, _ := uuid.Parse(c.Param("volume_id"))

	workflowID, err := h.svc.Unmount(c.Request().Context(), userID, projectID, containerID, volumeID)
	if err != nil {
		return response.Error(c, err)
	}
	return response.OK(c, map[string]string{"workflow_id": workflowID})
}

// ---- ConnectionHandler ----

type ConnectionHandler struct {
	svc service.ConnectionService
}

func NewConnectionHandler(svc service.ConnectionService) *ConnectionHandler {
	return &ConnectionHandler{svc: svc}
}

func (h *ConnectionHandler) List(c *echo.Context) error {
	userID := c.Get("UserID").(string)
	projectID, _ := uuid.Parse(c.Param("project_id"))

	conns, err := h.svc.List(c.Request().Context(), userID, projectID)
	if err != nil {
		return response.Error(c, err)
	}
	return response.OK(c, conns)
}

// ---- WebhookHandler ----

type WebhookHandler struct {
	svc service.ContainerService
}

func NewWebhookHandler(svc service.ContainerService) *WebhookHandler {
	return &WebhookHandler{svc: svc}
}

func (h *WebhookHandler) Receive(c *echo.Context) error {
	token := c.Param("token")
	if err := h.svc.HandleWebhook(c.Request().Context(), token); err != nil {
		return response.Error(c, err)
	}
	return response.OK(c, map[string]interface{}{})
}
