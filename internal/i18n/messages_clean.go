package i18n

func init() {
	register(LangEN, messagesEN_clean)
	register(LangJA, messagesJA_clean)
}

// messagesEN_clean mirrors locale/en.sh keys clean_*.
//
//nolint:gochecknoglobals,revive // catalog tables are file-scoped by design.
var messagesEN_clean = map[string]string{
	"clean_inside_container":   "This script cannot be run from inside a container",
	"clean_header":             "Docker Volume Cleanup Script",
	"clean_workspace":          "Workspace:",
	"clean_docker_not_found":   "docker command not found",
	"clean_docker_not_running": "Docker daemon is not running",
	"clean_project_name":       "Project name:",
	"clean_service_name":       "Service name:",
	"clean_volume_prefix":      "Volume prefix:",
	"clean_no_volumes":         "No volumes found to delete",
	"clean_prefix_info":        "  Prefix: %s",
	"clean_volumes_header":     "Volumes to delete (%s):",
	"clean_notice":             "⚠ Notice:",
	"clean_notice_1":           "  - All volumes listed above will be deleted",
	"clean_notice_2":           "  - Data in volumes cannot be recovered",
	"clean_notice_3":           "  - Stop running containers first",
	"clean_confirm":            "Proceed with deletion? [y/N]: ",
	"clean_cancelled":          "Cancelled",
	"clean_stopping":           "Removing containers...",
	"clean_deleting":           "Deleting volumes...",
	"clean_vol_failed":         "%s (deletion failed — may be in use)",
	"clean_all_deleted":        "✅ All %s volumes deleted successfully",
	"clean_partial":            "⚠ %s deleted, %s failed",
}

//nolint:gochecknoglobals,revive // catalog tables are file-scoped by design.
var messagesJA_clean = map[string]string{
	"clean_inside_container":   "このスクリプトはコンテナ内から実行できません",
	"clean_header":             "Docker ボリュームクリーンアップスクリプト",
	"clean_workspace":          "ワークスペース:",
	"clean_docker_not_found":   "docker コマンドが見つかりません",
	"clean_docker_not_running": "Docker デーモンが起動していません",
	"clean_project_name":       "プロジェクト名:",
	"clean_service_name":       "サービス名:",
	"clean_volume_prefix":      "ボリュームプレフィックス:",
	"clean_no_volumes":         "削除するボリュームがありません",
	"clean_prefix_info":        "  プレフィックス: %s",
	"clean_volumes_header":     "削除するボリューム (%s):",
	"clean_notice":             "⚠ 注意:",
	"clean_notice_1":           "  - 上記のボリュームがすべて削除されます",
	"clean_notice_2":           "  - ボリューム内のデータは復元できません",
	"clean_notice_3":           "  - 先に実行中のコンテナを停止してください",
	"clean_confirm":            "削除を実行しますか？ [y/N]: ",
	"clean_cancelled":          "キャンセルしました",
	"clean_stopping":           "コンテナを削除中...",
	"clean_deleting":           "ボリュームを削除中...",
	"clean_vol_failed":         "%s（削除失敗 — 使用中の可能性があります）",
	"clean_all_deleted":        "✅ %s 個のボリュームをすべて削除しました",
	"clean_partial":            "⚠ %s 個削除、%s 個失敗",
}
