package i18n

func init() {
	register(LangEN, messagesEN_rebuild)
	register(LangJA, messagesJA_rebuild)
}

// messagesEN_rebuild mirrors locale/en.sh keys rebuild_*.
//
//nolint:gochecknoglobals,revive // catalog tables are file-scoped by design.
var messagesEN_rebuild = map[string]string{
	"rebuild_inside_container": "This script cannot be run from inside a container",
	"rebuild_header":           "No-cache Rebuild Script",
	"rebuild_workspace":        "Workspace:",
	"rebuild_current_image":    "Current image: %s",
	"rebuild_created":          "Created:       %s (%s days ago)",
	"rebuild_image_not_found":  "Image %s not found (first build)",
	"rebuild_notice":           "⚠ Notice:",
	"rebuild_notice_1":         "  - The Docker image will be rebuilt without cache",
	"rebuild_notice_2":         "  - The existing container will be deleted and recreated",
	"rebuild_notice_3":         "  - The rebuild may take several minutes",
	"rebuild_confirm":          "Proceed with rebuild? [y/N]: ",
	"rebuild_cancelled":        "Cancelled",
	"rebuild_starting":         "🔨 Rebuilding without cache & starting...",
	"rebuild_please_wait":      "   This may take several minutes",
	"rebuild_complete":         "✅ Rebuild & startup complete",
	"rebuild_new_image":        "New image created: %s",
	"rebuild_vscode_1":         "📌 In VS Code, press Ctrl+Shift+P →",
	"rebuild_vscode_2":         "   'Dev Containers: Reopen in Container'",
}

//nolint:gochecknoglobals,revive // catalog tables are file-scoped by design.
var messagesJA_rebuild = map[string]string{
	"rebuild_inside_container": "このスクリプトはコンテナ内から実行できません",
	"rebuild_header":           "キャッシュなしリビルドスクリプト",
	"rebuild_workspace":        "ワークスペース:",
	"rebuild_current_image":    "現在のイメージ: %s",
	"rebuild_created":          "作成日時:       %s (%s 日前)",
	"rebuild_image_not_found":  "イメージ %s が見つかりません（初回ビルド）",
	"rebuild_notice":           "⚠ 注意:",
	"rebuild_notice_1":         "  - Docker イメージがキャッシュなしでリビルドされます",
	"rebuild_notice_2":         "  - 既存のコンテナは削除され再作成されます",
	"rebuild_notice_3":         "  - リビルドには数分かかる場合があります",
	"rebuild_confirm":          "リビルドを実行しますか？ [y/N]: ",
	"rebuild_cancelled":        "キャンセルしました",
	"rebuild_starting":         "🔨 キャッシュなしでリビルド＆起動中...",
	"rebuild_please_wait":      "   数分かかる場合があります",
	"rebuild_complete":         "✅ リビルド＆起動完了",
	"rebuild_new_image":        "新しいイメージを作成: %s",
	"rebuild_vscode_1":         "📌 VS Code で Ctrl+Shift+P を押して →",
	"rebuild_vscode_2":         "   'Dev Containers: Reopen in Container'",
}
