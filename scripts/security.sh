#!/usr/bin/env bash
# Security gate: triage rules for .trivyignore.yaml, then gitleaks (git history) and Trivy (trivy.yaml).
# Usage: scripts/security.sh   — needs gitleaks, trivy and yq from mise.toml; exits 1 on any finding.
set -uo pipefail

today="${SECURITY_TODAY:-$(date -u +%F)}"
latest="$(date -u -d "$today +31 days" +%F 2>/dev/null || date -u -j -v+31d -f %F "$today" +%F)"
status=0

# One line per exception: kind, index, id, statement, expired_at (tab-separated).
entries="$(yq -r '
  ["vulnerabilities", "secrets", "misconfigurations", "licenses"][] as $k
  | (.[$k] // [])[] as $e ireduce ({"i": -1, "out": ""}; .i += 1
    | .out += ([$k, .i, ($e.id // ""), ($e.statement // ""), ($e.expired_at // "")] | map(tostring) | join("\t")) + "\n")
  | .out' .trivyignore.yaml)" || { echo ".trivyignore.yaml: cannot be read" >&2; exit 1; }

while IFS=$'\t' read -r kind i id statement expiry; do
  [ -z "$kind" ] && continue
  at="$kind[$i] (${id:-no id})"
  problem() { echo ".trivyignore.yaml: $at: $1" >&2; status=1; }
  [ -n "$id" ] || problem "id is required"
  [ -n "${statement// /}" ] || problem "statement is required"
  if ! [[ "$expiry" =~ ^[0-9]{4}-[0-9]{2}-[0-9]{2}$ ]]; then problem "expired_at is required (YYYY-MM-DD)"
  elif [[ ! "$expiry" > "$today" ]]; then problem "expired on $expiry, triage again"
  elif [[ "$expiry" > "$latest" ]]; then problem "expired_at $expiry is more than 31 days ahead"
  fi
done <<< "$entries"

echo "$ gitleaks git --redact --no-banner ."
gitleaks git --redact --no-banner . || status=1
echo "$ trivy fs --config trivy.yaml ."
trivy fs --config trivy.yaml --skip-version-check . || status=1
exit "$status"
