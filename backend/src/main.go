package main

import (
	"fmt"
	"net/http"
	"os"
	"runtime/debug"
	"time"

	"backend/handler"
	"backend/middlewares"
	"backend/repository"
	"backend/service"
	"launchs/shared/config"
	"launchs/shared/database"
	"launchs/shared/model"

	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
	"go.temporal.io/sdk/client"
	"gorm.io/gorm"
)

func main() {
	// DB 初期化
	database.Init()
	db := database.DB

	// AutoMigrate で全テーブルを作成・更新します
	if err := runMigrate(db); err != nil {
		panic("failed to migrate: " + err.Error())
	}

	// JWT 公開鍵の読み込み
	middlewares.Init()

	// Temporal クライアント初期化
	temporalAddr := config.TemporalAddress()
	temporalClient, err := client.Dial(client.Options{
		HostPort: temporalAddr,
	})
	if err != nil {
		panic("failed to connect Temporal: " + err.Error())
	}
	defer temporalClient.Close()

	// テンプレートディレクトリ（git submodule）
	templateDir := os.Getenv("TEMPLATE_DIR")
	if templateDir == "" {
		templateDir = "/templates"
	}

	// Repository 初期化（DI）
	projectRepo := repository.NewProjectRepository(db)
	containerRepo := repository.NewContainerRepository(db)
	volumeRepo := repository.NewVolumeRepository(db)
	envVarRepo := repository.NewEnvVarRepository(db)
	portRepo := repository.NewPortRepository(db)
	routeRepo := repository.NewNetworkRouteRepository(db)
	logRepo := repository.NewLogRepository(db)
	metricRepo := repository.NewMetricRepository(db)
	buildJobRepo := repository.NewBuildJobRepository(db)
	snapshotRepo := repository.NewSnapshotRepository(db)
	connectionRepo := repository.NewServiceConnectionRepository(db)
	statusHistRepo := repository.NewContainerStatusHistoryRepository(db)
	userQuotaRepo := repository.NewUserQuotaRepository(db)

	// Service 初期化（DI）
	projectSvc := service.NewProjectService(projectRepo, containerRepo, buildJobRepo, snapshotRepo, temporalClient)
	templateSvc := service.NewTemplateService(templateDir)
	quotaSvc := service.NewQuotaService(userQuotaRepo, containerRepo)
	containerSvc := service.NewContainerService(projectRepo, containerRepo, envVarRepo, portRepo, buildJobRepo, volumeRepo, routeRepo, templateSvc, quotaSvc, temporalClient)
	envVarSvc := service.NewEnvVarService(projectRepo, containerRepo, envVarRepo)
	portSvc := service.NewPortService(projectRepo, containerRepo, portRepo)
	routeSvc := service.NewRouteService(projectRepo, containerRepo, routeRepo, temporalClient)
	volumeSvc := service.NewVolumeService(projectRepo, containerRepo, volumeRepo, temporalClient)
	logSvc := service.NewLogService(projectRepo, containerRepo, buildJobRepo, logRepo)
	metricSvc := service.NewMetricService(projectRepo, containerRepo, metricRepo)
	buildJobSvc := service.NewBuildJobService(projectRepo, containerRepo, buildJobRepo, temporalClient)
	snapshotSvc := service.NewSnapshotService(projectRepo, containerRepo, snapshotRepo, temporalClient)
	connectionSvc := service.NewConnectionService(projectRepo, connectionRepo)

	// Handler 初期化
	projectH := handler.NewProjectHandler(projectSvc)
	containerH := handler.NewContainerHandler(containerSvc, statusHistRepo)
	envVarH := handler.NewEnvVarHandler(envVarSvc)
	portH := handler.NewPortHandler(portSvc)
	routeH := handler.NewRouteHandler(routeSvc)
	volumeH := handler.NewVolumeHandler(volumeSvc)
	logH := handler.NewLogHandler(logSvc)
	metricH := handler.NewMetricHandler(metricSvc)
	buildJobH := handler.NewBuildJobHandler(buildJobSvc)
	templateH := handler.NewTemplateHandler(templateSvc)
	snapshotH := handler.NewSnapshotHandler(snapshotSvc)
	connectionH := handler.NewConnectionHandler(connectionSvc)
	webhookH := handler.NewWebhookHandler(containerSvc)
	quotaH := handler.NewQuotaHandler(quotaSvc)

	// Echo ルーター設定
	e := echo.New()

	// リクエストログ: メソッド・パス・ステータス・レイテンシを標準出力へ
	e.Use(middleware.RequestLoggerWithConfig(middleware.RequestLoggerConfig{
		LogMethod:  true,
		LogURI:     true,
		LogStatus:  true,
		LogLatency: true,
		LogValuesFunc: func(c *echo.Context, v middleware.RequestLoggerValues) error {
			fmt.Printf("[REQ] %s %s status=%d latency=%s\n",
				v.Method, v.URI, v.Status, v.Latency.Round(time.Millisecond))
			return nil
		},
	}))

	// パニック時にスタックトレースを標準出力へ出してから 500 を返す
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) (err error) {
			defer func() {
				if r := recover(); r != nil {
					fmt.Printf("[PANIC] %v\n%s\n", r, debug.Stack())
					err = c.JSON(http.StatusInternalServerError, map[string]interface{}{
						"data":  nil,
						"error": map[string]string{"code": "PANIC", "message": "internal server error"},
					})
				}
			}()
			return next(c)
		}
	})

	e.Use(middleware.Recover())

	// ヘルスチェック（認証不要）
	e.GET("/health", func(c *echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	// テンプレート一覧（認証不要）
	e.GET("/api/v1/templates", templateH.List)
	e.GET("/api/v1/templates/:template_name", templateH.Get)

	// フロントエンド向け設定値（認証不要）
	e.GET("/api/v1/config", func(c *echo.Context) error {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"data": map[string]interface{}{
				"max_volume_size_mb": config.GetMaxVolumeSizeMB(),
				"min_volume_size_mb": config.GetMinVolumeSizeMB(),
			},
			"error": nil,
		})
	})

	// Webhook 受信（認証不要・トークンで識別）
	e.POST("/api/v1/webhooks/:token", webhookH.Receive)

	// 認証が必要な API グループ
	v1 := e.Group("/api/v1", middlewares.RequireAuth)

	// Projects
	v1.GET("/projects", projectH.List)
	v1.POST("/projects", projectH.Create)
	v1.GET("/projects/:project_id", projectH.Get)
	v1.DELETE("/projects/:project_id", projectH.Delete)
	v1.POST("/projects/:project_id/deploy", projectH.Deploy)
	v1.GET("/projects/:project_id/jobs", projectH.GetJobs)

	// Project Env Vars
	v1.GET("/projects/:project_id/env-vars", envVarH.ListProject)
	v1.PUT("/projects/:project_id/env-vars", envVarH.UpsertProject)
	v1.DELETE("/projects/:project_id/env-vars", envVarH.DeleteProject)

	// クォータ確認（認証必要）
	v1.GET("/quota", quotaH.GetMyQuota)

	// Containers
	v1.GET("/projects/:project_id/containers", containerH.List)
	v1.GET("/projects/:project_id/containers/:container_id", containerH.Get)
	v1.POST("/projects/:project_id/containers/deploy", containerH.BuildDeploy)
	v1.POST("/projects/:project_id/containers/deploy-image", containerH.DeployImage)
	v1.POST("/projects/:project_id/containers/from-template", containerH.FromTemplate)
	v1.PUT("/projects/:project_id/containers/:container_id", containerH.Update)
	v1.DELETE("/projects/:project_id/containers/:container_id", containerH.Delete)
	v1.POST("/projects/:project_id/containers/:container_id/redeploy", containerH.Redeploy)
	v1.POST("/projects/:project_id/containers/:container_id/rebuild", containerH.Rebuild)
	v1.PUT("/projects/:project_id/containers/:container_id/scale", containerH.Scale)
	v1.POST("/projects/:project_id/containers/:container_id/webhook", containerH.CreateWebhook)
	v1.GET("/projects/:project_id/containers/:container_id/status-histories", containerH.GetStatusHistories)

	// Container Env Vars
	v1.GET("/projects/:project_id/containers/:container_id/env-vars", envVarH.ListContainer)
	v1.PUT("/projects/:project_id/containers/:container_id/env-vars", envVarH.UpsertContainer)
	v1.DELETE("/projects/:project_id/containers/:container_id/env-vars", envVarH.DeleteContainer)
	v1.GET("/projects/:project_id/containers/:container_id/selected-project-env-vars", envVarH.GetSelectedProjectEnvVarKeys)
	v1.PUT("/projects/:project_id/containers/:container_id/selected-project-env-vars", envVarH.SetSelectedProjectEnvVarKeys)

	// Ports
	v1.GET("/projects/:project_id/containers/:container_id/ports", portH.List)
	v1.POST("/projects/:project_id/containers/:container_id/ports", portH.Create)
	v1.DELETE("/projects/:project_id/containers/:container_id/ports/:port_id", portH.Delete)

	// Routes
	v1.GET("/projects/:project_id/containers/:container_id/routes", routeH.List)
	v1.POST("/projects/:project_id/containers/:container_id/routes/service", routeH.CreateService)
	v1.POST("/projects/:project_id/containers/:container_id/routes/ingress", routeH.CreateIngress)
	v1.DELETE("/projects/:project_id/containers/:container_id/routes/:route_id", routeH.Delete)

	// Volume Mounts
	v1.POST("/projects/:project_id/containers/:container_id/mounts", volumeH.Mount)
	v1.DELETE("/projects/:project_id/containers/:container_id/mounts/:volume_id", volumeH.Unmount)

	// Logs
	v1.GET("/projects/:project_id/containers/:container_id/logs", logH.GetContainer)
	v1.GET("/projects/:project_id/build-jobs/:build_job_id/logs", logH.GetBuildJob)

	// Metrics
	v1.GET("/projects/:project_id/containers/:container_id/metrics", metricH.Get)

	// Build Jobs
	v1.GET("/projects/:project_id/containers/:container_id/build-jobs", buildJobH.List)
	v1.DELETE("/projects/:project_id/build-jobs/:build_job_id", buildJobH.Cancel)

	// Volumes
	v1.GET("/projects/:project_id/volumes", volumeH.List)
	v1.POST("/projects/:project_id/volumes", volumeH.Create)
	v1.DELETE("/projects/:project_id/volumes/:volume_id", volumeH.Delete)

	// Snapshots
	v1.GET("/projects/:project_id/snapshots", snapshotH.List)
	v1.GET("/projects/:project_id/snapshots/:snapshot_id", snapshotH.Get)
	v1.POST("/projects/:project_id/snapshots/:snapshot_id/restore", snapshotH.Restore)

	// Service Connections（フロー可視化）
	v1.GET("/projects/:project_id/connections", connectionH.List)

	// 管理者API（X-Admin-Key ヘッダーで認証）
	admin := e.Group("/api/v1/admin", middlewares.RequireAdminKey)
	admin.PUT("/users/:user_id/quota", quotaH.SetQuota)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	if err := e.Start(":" + port); err != nil {
		e.Logger.Error("server error", "error", err)
	}
}

// runMigrate は AutoMigrate で全テーブルを作成・更新します。
func runMigrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&model.UserQuota{},
		&model.Project{},
		&model.Container{},
		&model.PodStatus{},
		&model.ContainerStatusHistory{},
		&model.ProjectEnvVar{},
		&model.ContainerEnvVar{},
		&model.ContainerSelectedProjectEnvVar{},
		&model.Port{},
		&model.NetworkRoute{},
		&model.Volume{},
		&model.VolumeMount{},
		&model.BuildJob{},
		&model.Image{},
		&model.Deployment{},
		&model.ContainerLog{},
		&model.ContainerMetric{},
		&model.Snapshot{},
		&model.ServiceConnection{},
	)
}

