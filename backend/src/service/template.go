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
	Key          string  `json:"key"`
	Required     bool    `json:"required"`
	Description  string  `json:"description"`
	AutoGenerate bool    `json:"auto_generate"`
	GenerateType *string `json:"generate_type"`
	Default      string  `json:"default"`
}

// TemplateVolumeDef はテンプレートが要求するボリュームの定義です。
type TemplateVolumeDef struct {
	Required      bool   `json:"required"`
	MountPath     string `json:"mount_path"`
	DefaultSizeMB int    `json:"default_size_mb"`
}

// TemplateDetail はテンプレートの詳細情報です。
type TemplateDetail struct {
	TemplateSummary
	EnvVars []TemplateEnvVarDef `json:"env_vars"`
	Volume  *TemplateVolumeDef  `json:"volume"`
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
}
