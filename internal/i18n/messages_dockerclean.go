package i18n

func init() {
	register(LangEN, messagesEN_dockerClean)
	register(LangJA, messagesJA_dockerClean)
}

// messagesEN_dockerClean mirrors locale/en.sh keys docker_clean_*.
//
//nolint:gochecknoglobals,revive // catalog tables are file-scoped by design.
var messagesEN_dockerClean = map[string]string{
	"docker_clean_inside_container":   "This script cannot be run from inside a container",
	"docker_clean_header":             "Docker Resource Cleanup",
	"docker_clean_not_found":          "docker command not found",
	"docker_clean_not_running":        "Docker daemon is not running",
	"docker_clean_disk_usage":         "Current Docker disk usage:",
	"docker_clean_disk_usage_after":   "Updated Docker disk usage:",
	"docker_clean_select_title":       "Select resources to clean up:",
	"docker_clean_opt_containers":     "Stopped containers (docker container prune)",
	"docker_clean_opt_builder":        "Build cache (docker builder prune)",
	"docker_clean_opt_images":         "Dangling images (docker image prune)",
	"docker_clean_opt_networks":       "Unused networks (docker network prune)",
	"docker_clean_opt_volumes":        "Unused volumes (docker volume prune) ⚠ DATA LOSS RISK",
	"docker_clean_cancelled":          "Cancelled",
	"docker_clean_running_containers": "Removing stopped containers...",
	"docker_clean_running_builder":    "Removing build cache...",
	"docker_clean_running_images":     "Removing dangling images...",
	"docker_clean_running_networks":   "Removing unused networks...",
	"docker_clean_running_volumes":    "Removing unused volumes...",
	"docker_clean_done_containers":    "Stopped containers removed",
	"docker_clean_done_builder":       "Build cache removed",
	"docker_clean_done_images":        "Dangling images removed",
	"docker_clean_done_networks":      "Unused networks removed",
	"docker_clean_done_volumes":       "Unused volumes removed",
	"docker_clean_fail_containers":    "Failed to remove stopped containers",
	"docker_clean_fail_builder":       "Failed to remove build cache",
	"docker_clean_fail_images":        "Failed to remove dangling images",
	"docker_clean_fail_networks":      "Failed to remove unused networks",
	"docker_clean_fail_volumes":       "Failed to remove unused volumes",
	"docker_clean_all_done":           "✅ All %s cleanup operations completed",
	"docker_clean_partial_done":       "⚠ %s succeeded, %s failed",
}

//nolint:gochecknoglobals,revive // catalog tables are file-scoped by design.
var messagesJA_dockerClean = map[string]string{
	"docker_clean_inside_container":   "このスクリプトはコンテナ内から実行できません",
	"docker_clean_header":             "Docker リソースクリーンアップ",
	"docker_clean_not_found":          "docker コマンドが見つかりません",
	"docker_clean_not_running":        "Docker デーモンが起動していません",
	"docker_clean_disk_usage":         "現在の Docker ディスク使用量:",
	"docker_clean_disk_usage_after":   "クリーンアップ後の Docker ディスク使用量:",
	"docker_clean_select_title":       "クリーンアップするリソースを選択:",
	"docker_clean_opt_containers":     "停止済みコンテナ (docker container prune)",
	"docker_clean_opt_builder":        "ビルドキャッシュ (docker builder prune)",
	"docker_clean_opt_images":         "不要イメージ (docker image prune)",
	"docker_clean_opt_networks":       "未使用ネットワーク (docker network prune)",
	"docker_clean_opt_volumes":        "未使用ボリューム (docker volume prune) ⚠ データ消失リスク",
	"docker_clean_cancelled":          "キャンセルしました",
	"docker_clean_running_containers": "停止済みコンテナを削除中...",
	"docker_clean_running_builder":    "ビルドキャッシュを削除中...",
	"docker_clean_running_images":     "不要イメージを削除中...",
	"docker_clean_running_networks":   "未使用ネットワークを削除中...",
	"docker_clean_running_volumes":    "未使用ボリュームを削除中...",
	"docker_clean_done_containers":    "停止済みコンテナを削除しました",
	"docker_clean_done_builder":       "ビルドキャッシュを削除しました",
	"docker_clean_done_images":        "不要イメージを削除しました",
	"docker_clean_done_networks":      "未使用ネットワークを削除しました",
	"docker_clean_done_volumes":       "未使用ボリュームを削除しました",
	"docker_clean_fail_containers":    "停止済みコンテナの削除に失敗しました",
	"docker_clean_fail_builder":       "ビルドキャッシュの削除に失敗しました",
	"docker_clean_fail_images":        "不要イメージの削除に失敗しました",
	"docker_clean_fail_networks":      "未使用ネットワークの削除に失敗しました",
	"docker_clean_fail_volumes":       "未使用ボリュームの削除に失敗しました",
	"docker_clean_all_done":           "✅ 全 %s 件のクリーンアップ操作が完了しました",
	"docker_clean_partial_done":       "⚠ %s 件完了、%s 件失敗",
}
