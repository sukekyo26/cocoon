package aptbase

// MinimalBasePackages is the strict minimum apt set every cocoon-built
// image installs unconditionally. It is intentionally tiny: anything
// the user wants beyond this goes through `cocoon init`'s
// AptCategories picker (or by hand in [apt] packages).
//
// The four entries cover the boot sequence the plugin install scripts
// rely on:
//
//   - ca-certificates: required for any HTTPS in plugin install.sh
//     scripts, including curl|sh installers and apt mirrors over HTTPS.
//   - curl:            same plugins use it directly to fetch tarballs.
//   - locales:         locale-gen at image build is otherwise broken.
//   - sudo:            many plugins drop privileges with `sudo -u`,
//     and `cocoon exec` also relies on it for the container user.
//
// The wider set of utility packages workspace-docker shipped in
// config/apt-base-packages.conf (vim, jq, fzf, ripgrep, ...) is
// deliberately not included; users opt in via [apt] packages.
//
//nolint:gochecknoglobals // tabular configuration data, file-scoped by design.
var MinimalBasePackages = []string{
	"ca-certificates",
	"curl",
	"locales",
	"sudo",
}
