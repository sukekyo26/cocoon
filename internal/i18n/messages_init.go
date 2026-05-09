package i18n

func init() {
	register(LangEN, messagesEN_init)
	register(LangJA, messagesJA_init)
}

// messagesEN_init holds prompt / status strings emitted by `cocoon init`
// and `cocoon gen`. Keys are deliberately scoped with `init_` / `gen_`
// prefixes so they do not collide with the legacy `setup_*` table.
//
//nolint:gochecknoglobals,revive // catalog tables are file-scoped by design.
var messagesEN_init = map[string]string{
	// init prompts
	"init_prompt_service_name": "Service name",
	"init_desc_service_name":   "Compose service name; lowercase letters, digits, _ and - only.",
	"init_err_service_name":    "must match %s",
	"init_prompt_username":     "Username",
	"init_desc_username":       "In-container user; lowercase letters, digits, _ and - only.",
	"init_err_username":        "must match %s",
	"init_prompt_os":           "Base OS",
	"init_desc_os":             "Linux distribution that backs the container image (FROM <os>:<os_version>).",
	"init_prompt_os_version":   "%s version",
	"init_desc_os_version":     "Pulled as FROM %s:<version> in the generated Dockerfile.",
	"init_prompt_mount_root":   "Mount range",
	"init_desc_mount_root":     "How much of your filesystem should be visible inside the container?",
	"init_option_mount_cwd":    "Just this project (.)",
	"init_option_mount_parent": "Parent directory — sibling repos visible (..)",
	"init_prompt_devcontainer": "Generate .devcontainer/devcontainer.json for VS Code Dev Containers?",
	"init_desc_devcontainer":   "Says yes if you ever open this repo in VS Code Dev Containers; harmless otherwise.",
	"init_prompt_apt":          "Select common apt packages to install",
	"init_desc_apt":            "Pre-checked categories are installed by default; uncheck what you do not need.",
	// init result + next steps
	"init_wrote":             "wrote %s",
	"init_next_header":       "Next steps:",
	"init_next_step_gen":     "  1. cocoon gen",
	"init_next_step_compose": "  2. docker compose -f .devcontainer/docker-compose.yml up -d",
	"init_next_step_vscode":  `     (or open in VS Code → "Reopen in Container")`,
	// gen result + next steps
	"gen_wrote":             "wrote %s",
	"gen_next_header":       "To start the container:",
	"gen_next_step_compose": "  docker compose -f .devcontainer/docker-compose.yml up -d",
	"gen_next_step_vscode":  `  (or open in VS Code → "Reopen in Container")`,
}

// messagesJA_init mirrors messagesEN_init in Japanese. Untranslated keys
// (typically command-name examples like `cocoon gen`) intentionally keep
// the English form because the command name itself is not localized.
//
//nolint:gochecknoglobals,revive // catalog tables are file-scoped by design.
var messagesJA_init = map[string]string{
	// init prompts
	"init_prompt_service_name": "サービス名",
	"init_desc_service_name":   "Compose のサービス名。英小文字・数字・_・- のみ使用可。",
	"init_err_service_name":    "%s に一致する必要があります",
	"init_prompt_username":     "ユーザー名",
	"init_desc_username":       "コンテナ内ユーザー名。英小文字・数字・_・- のみ使用可。",
	"init_err_username":        "%s に一致する必要があります",
	"init_prompt_os":           "ベース OS",
	"init_desc_os":             "コンテナイメージのベース Linux ディストリビューション (FROM <os>:<os_version>)。",
	"init_prompt_os_version":   "%s のバージョン",
	"init_desc_os_version":     "生成される Dockerfile の FROM %s:<version> に展開されます。",
	"init_prompt_mount_root":   "マウント範囲",
	"init_desc_mount_root":     "コンテナ内に見せるファイルシステムの範囲を選んでください。",
	"init_option_mount_cwd":    "このプロジェクトのみ (.)",
	"init_option_mount_parent": "親ディレクトリ — 兄弟リポジトリも見える (..)",
	"init_prompt_devcontainer": ".devcontainer/devcontainer.json を VS Code Dev Containers 用に生成しますか？",
	"init_desc_devcontainer":   "VS Code Dev Containers で開く可能性があれば Yes。そうでなくても害はありません。",
	"init_prompt_apt":          "インストールする apt パッケージのカテゴリを選択",
	"init_desc_apt":            "プリチェック済みのカテゴリがデフォルトでインストールされます。不要なものはチェックを外してください。",
	// init result + next steps
	"init_wrote":             "%s を書き出しました",
	"init_next_header":       "次のステップ:",
	"init_next_step_gen":     "  1. cocoon gen",
	"init_next_step_compose": "  2. docker compose -f .devcontainer/docker-compose.yml up -d",
	"init_next_step_vscode":  `     (または VS Code で「Reopen in Container」を実行)`,
	// gen result + next steps
	"gen_wrote":             "%s を書き出しました",
	"gen_next_header":       "コンテナを起動するには:",
	"gen_next_step_compose": "  docker compose -f .devcontainer/docker-compose.yml up -d",
	"gen_next_step_vscode":  `  (または VS Code で「Reopen in Container」を実行)`,
}
