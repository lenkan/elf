server=http://127.0.0.1:4096
cd "$1"

for i in $(seq 120); do
	if opencode --server "$server" api get /api/info >/dev/null 2>&1; then
		break
	fi
	sleep 0.5
done

if opencode --server "$server" session list --format json --max-count 1 | grep -q '"id"'; then
	exec opencode --server "$server" --continue .
fi
exec opencode --server "$server" .
