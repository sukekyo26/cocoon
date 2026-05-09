// Package generate produces Dockerfile, docker-compose.yml, devcontainer.json
// and the bashrc fragment from a normalized workspace context. Each artifact
// lives in its own subpackage so the per-artifact code stays in reviewable
// units.
package generate
