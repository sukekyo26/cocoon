package warn

// Diagnostic codes. Each constant equals the i18n catalog key the drain site
// resolves; TestWarnCodesHaveCatalogEntries (internal/i18n) asserts every code
// here exists in both language tables. Codes commented "Ref" are used as
// nested [Ref] args, not as a top-level [Warning.Code].
const (
	// Volume dedup (internal/generate/compose).
	//nolint:gosec // G101 false positive: i18n catalog key, not a credential.
	VolumeDupWorkspace = "warn_volume_dup_workspace"
	VolLabelPlugin     = "warn_vol_label_plugin"    // Ref
	VolLabelWorkspace  = "warn_vol_label_workspace" // Ref

	// Dockerfile (internal/generate/dockerfile).
	AptRedundant       = "warn_apt_redundant"
	DockerfileVerbatim = "warn_dockerfile_verbatim"

	// Plugin version pins (internal/generate/dockerfile).
	PinNoChecksum = "warn_pin_no_checksum"
	PinNoVerify   = "warn_pin_no_verify"

	// Locale (internal/generate).
	TZOverride = "warn_tz_override"

	// Ports skipped for devcontainer.json forwardPorts (internal/config).
	PortSkip                   = "warn_port_skip"
	PortReasonType             = "warn_port_reason_type"              // Ref
	PortReasonShortInvalid     = "warn_port_reason_short_invalid"     // Ref
	PortReasonShortUDP         = "warn_port_reason_short_udp"         // Ref
	PortReasonRange            = "warn_port_reason_range"             // Ref
	PortReasonUnparseable      = "warn_port_reason_unparseable"       // Ref
	PortReasonHostMode         = "warn_port_reason_host_mode"         // Ref
	PortReasonLongUDP          = "warn_port_reason_long_udp"          // Ref
	PortReasonMissingTarget    = "warn_port_reason_missing_target"    // Ref
	PortReasonNonIntegerTarget = "warn_port_reason_noninteger_target" // Ref

	// Plugin loader / layering (internal/plugin).
	PluginNotFound   = "warn_plugin_not_found"
	PluginOverridden = "info_plugin_overridden"
)

// Codes returns every diagnostic code (including Ref-only ones) so the i18n
// parity test can assert each has a catalog entry in both languages.
func Codes() []string {
	return []string{
		VolumeDupWorkspace, VolLabelPlugin, VolLabelWorkspace,
		AptRedundant, DockerfileVerbatim,
		PinNoChecksum, PinNoVerify,
		TZOverride,
		PortSkip, PortReasonType, PortReasonShortInvalid, PortReasonShortUDP,
		PortReasonRange, PortReasonUnparseable, PortReasonHostMode,
		PortReasonLongUDP, PortReasonMissingTarget, PortReasonNonIntegerTarget,
		PluginNotFound, PluginOverridden,
	}
}
