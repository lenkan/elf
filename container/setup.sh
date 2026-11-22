set -e
mkdir -p ~/.config/opencode
git init --quiet ~/work/repo
git -C ~/work/repo symbolic-ref HEAD "refs/heads/$1"
# Your pushes update the working tree while it's clean
git -C ~/work/repo config receive.denyCurrentBranch updateInstead
printf '%s\n' "$ELF_INSTRUCTIONS" > ~/work/AGENTS.md
