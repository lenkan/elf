set -e
printf '%s' "$ELF_CREDENTIALS" | opencode --standalone auth import >/dev/null
unset ELF_CREDENTIALS
exec opencode serve --hostname 127.0.0.1 --port 4096
