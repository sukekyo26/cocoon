package i18n

// Generator-pipeline diagnostics (warnings / informational notes) collected as
// structured codes by internal/warn and rendered here at the CLI boundary.
// Keys mirror the constants in internal/warn/codes.go; warn_*_label / warn_port_reason_*
// entries are nested sub-messages (warn.Ref) interpolated into a parent key.

func init() {
	register(LangEN, warningsEN)
	register(LangJA, warningsJA)
}

var warningsEN = map[string]string{
	// Volume dedup.
	"warn_volume_dup_workspace": "WARNING: Volume path '%s' is defined by both %s and config volume '%s'. Using single volume '%s'.",
	"warn_vol_label_plugin":     "plugin '%s'",
	"warn_vol_label_workspace":  "config volume '%s'",

	// Dockerfile.
	"warn_apt_redundant":       "WARNING: [apt] packages contains '%s', which cocoon already installs as a base package. Remove it from [apt] packages in your config file to avoid redundant installs.",
	"warn_dockerfile_verbatim": "WARNING: Custom Dockerfile instructions from [dockerfile].%s are being injected verbatim. You are responsible for their safety.",

	// Plugin version pins.
	"warn_pin_no_checksum": "WARNING: '%s' is pinned to %q without a recorded checksum; the install step still verifies the download against the upstream-published checksum. Run `cocoon lock` to record it for reproducible builds.",
	"warn_pin_no_verify":   "WARNING: '%s' is pinned to %q but its upstream publishes no checksum, so the install step downloads it WITHOUT verification. Set checksum_amd64/checksum_arm64 in [plugins.options].%s to verify.",

	// Locale.
	"warn_tz_override": "WARNING: [env].TZ='%s' is overridden by [locale].timezone='%s'.",

	// Ports skipped for devcontainer.json forwardPorts.
	"warn_port_skip":                     "WARNING: ports.forward[%d] %s; skipping for devcontainer.json forwardPorts.",
	"warn_port_reason_type":              "has unsupported type %s",
	"warn_port_reason_short_invalid":     "= %q is not a valid short-form port",
	"warn_port_reason_short_udp":         "= %q uses protocol = \"udp\"",
	"warn_port_reason_range":             "= %q uses a port range",
	"warn_port_reason_unparseable":       "= %q has unparseable port",
	"warn_port_reason_host_mode":         "uses mode = \"host\"",
	"warn_port_reason_long_udp":          "uses protocol = \"udp\"",
	"warn_port_reason_missing_target":    "is missing target",
	"warn_port_reason_noninteger_target": "has a non-integer target %v",

	// Plugin loader / layering.
	"warn_plugin_not_found":  "WARNING: Plugin '%s' not found at %s",
	"info_plugin_overridden": "INFO: plugin %s overridden by %s",
}

var warningsJA = map[string]string{
	// Volume dedup.
	"warn_volume_dup_workspace": "警告: ボリュームパス '%s' が %s と設定ファイルの volume '%s' の両方で定義されています。単一のボリューム '%s' を使用します。",
	"warn_vol_label_plugin":     "プラグイン '%s'",
	"warn_vol_label_workspace":  "設定ファイルの volume '%s'",

	// Dockerfile.
	"warn_apt_redundant":       "警告: [apt] packages に '%s' が含まれていますが、cocoon がベースパッケージとして既にインストールします。重複インストールを避けるため設定ファイルの [apt] packages から削除してください。",
	"warn_dockerfile_verbatim": "警告: [dockerfile].%s のカスタム Dockerfile 命令はそのまま埋め込まれます。その安全性はあなたの責任です。",

	// Plugin version pins.
	"warn_pin_no_checksum": "警告: '%s' は checksum の記録なしに %q へ pin されています。install ステップは upstream が公開する checksum でダウンロードを検証します。再現可能ビルドのため `cocoon lock` で記録してください。",
	"warn_pin_no_verify":   "警告: '%s' は %q へ pin されていますが upstream が checksum を公開していないため、install ステップは検証なしでダウンロードします。検証するには [plugins.options].%s に checksum_amd64/checksum_arm64 を設定してください。",

	// Locale.
	"warn_tz_override": "警告: [env].TZ='%s' は [locale].timezone='%s' によって上書きされます。",

	// Ports skipped for devcontainer.json forwardPorts.
	"warn_port_skip":                     "警告: ports.forward[%d] %s。devcontainer.json の forwardPorts からスキップします。",
	"warn_port_reason_type":              "はサポートされない型 %s です",
	"warn_port_reason_short_invalid":     "= %q は有効な短縮形ポートではありません",
	"warn_port_reason_short_udp":         "= %q は protocol = \"udp\" を使用しています",
	"warn_port_reason_range":             "= %q はポート範囲を使用しています",
	"warn_port_reason_unparseable":       "= %q はパースできないポートです",
	"warn_port_reason_host_mode":         "は mode = \"host\" を使用しています",
	"warn_port_reason_long_udp":          "は protocol = \"udp\" を使用しています",
	"warn_port_reason_missing_target":    "は target が指定されていません",
	"warn_port_reason_noninteger_target": "は整数でない target %v を持っています",

	// Plugin loader / layering.
	"warn_plugin_not_found":  "警告: プラグイン '%s' が %s に見つかりません",
	"info_plugin_overridden": "情報: プラグイン %s は %s によって上書きされます",
}
