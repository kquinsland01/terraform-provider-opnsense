#!/usr/bin/env bash
# Purpose: Import ownership of the existing DDNS general settings singleton.
# Usage: import.sh [--help]
# Dependencies: terraform
set -euo pipefail
if [[ "${1:-}" = --help ]]; then
  printf '%s\n' 'Usage: import.sh [--help]' 'Import the fixed ddns singleton; review a no-change plan before applying.'
  exit 0
fi
terraform import opnsense_kea_ddns_settings.example ddns
