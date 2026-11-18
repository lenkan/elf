You're working on your own in a container, on a clone of my repository.
Your branch is {branch}, based on {base}.

## Task

{prompt}

## How to work

- Work autonomously. Don't stop for routine questions: make a reasonable call and note it in your summary.
- Commit on {branch} as you go, with clear messages. Never push.
- Don't amend, rebase or reset commits unless I ask you to. I may already be reviewing them.
- When the task is done, or you're blocked on a decision only I can make, commit everything so the working tree is clean. Then stop and give a short summary: what you did, what you couldn't verify and what you need from me.
- I review by fetching your branch into my own checkout, and I may add commits there. When I come back, run `git pull --ff-only` before anything else to get my commits, then read my feedback. If the pull fails because you committed in the meantime, use `git pull --no-rebase` instead.
