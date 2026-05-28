package service

// TemplateSummary はテンプレート一覧の各エントリです。
type TemplateSummary struct {
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	Category    string `json:"category"`
	Description string `json:"description"`
	Version     string `json:"version"`
	Icon        string `json:"icon"`
	Color       string `json:"color"`
}

// TemplateEnvVarDef はテンプレートが要求する環境変数の定義です。
type TemplateEnvVarDef struct {
	Key          string  `json:"key" yaml:"key"`
	Required     bool    `json:"required" yaml:"required"`
	Description  string  `json:"description" yaml:"description"`
	AutoGenerate bool    `json:"auto_generate" yaml:"auto_generate"`
	GenerateType *string `json:"generate_type" yaml:"generate_type"`
	Default      string  `json:"default" yaml:"default"`
}

// TemplateVolumeDef はテンプレートが要求するボリュームの定義です。
type TemplateVolumeDef struct {
	Required      bool   `json:"required" yaml:"required"`
	MountPath     string `json:"mount_path" yaml:"mount_path"`
	DefaultSizeMB int    `json:"default_size_mb" yaml:"default_size_mb"`
}

// TemplatePortDef はテンプレートが公開するポートの定義です。
type TemplatePortDef struct {
	Port     int    `json:"port" yaml:"port"`
	Protocol string `json:"protocol" yaml:"protocol"`
}

// TemplateSpecDef はテンプレートのデフォルトデプロイスペックです。
type TemplateSpecDef struct {
	ResourceSize string `json:"resource_size" yaml:"resource_size"`
	Replicas     int    `json:"replicas" yaml:"replicas"`
}

// TemplateDetail はテンプレートの詳細情報です。
type TemplateDetail struct {
	TemplateSummary
	Image   string             `json:"image"`
	EnvVars []TemplateEnvVarDef `json:"env_vars"`
	Volume  *TemplateVolumeDef  `json:"volume"`
	Ports   []TemplatePortDef   `json:"ports"`
	Spec    *TemplateSpecDef    `json:"spec"`
}

// TemplateFile は git submodule のテンプレート YAML の構造です。
type TemplateFile struct {
	Name        string             `yaml:"name"`
	DisplayName string             `yaml:"display_name"`
	Category    string             `yaml:"category"`
	Description string             `yaml:"description"`
	Version     string             `yaml:"version"`
	Icon        string             `yaml:"icon"`
	Color       string             `yaml:"color"`
	Image       string             `yaml:"image"`
	EnvVars     []TemplateEnvVarDef `yaml:"env_vars"`
	Volume      *TemplateVolumeDef  `yaml:"volume"`
	Ports       []TemplatePortDef   `yaml:"ports"`
	Spec        *TemplateSpecDef    `yaml:"spec"`
}
