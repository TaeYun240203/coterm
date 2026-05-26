---
name: coterm-shared-terminal
description: Use coterm to execute all task commands inside the user's shared visible tmux terminal.
---

# coterm shared terminal

You must execute commands only by invoking `coterm`.

No other executable may be invoked directly for the user's task. This prohibition applies even if the executable is not listed in examples.

Do not directly run commands such as `npm`, `git`, `python`, `go`, `ssh`, `tmux`, `cat`, `sed`, `rg`, `ls`, tests, builds, package managers, project tools, or any other executable.

If any command is needed, run it through:

`coterm run --pane <pane> -- <command> ...`

The only executable you may invoke directly is `coterm`.

Allowed direct invocations are only:

- `coterm sync`
- `coterm run`
- `coterm read`
- `coterm snapshot`
- `coterm panes`
- `coterm pane close`
- `coterm debug`
- `coterm export` only when the user explicitly asks for logs or export

Workflow:

- Call `coterm sync` before every `coterm run`.
- Call `coterm sync` after every `coterm run`.
- Call `coterm sync` before the final answer.
- Preserve and reuse the returned `client_id`.
- Choose a pane explicitly.
- Prefer `main1`, `main2`, `scratch1`, `scratch2`, `longrun1`, `longrun2`, `interactive1`, and `interactive2`.
- Use `--stdin` for multiline scripts.
- Do not inspect, edit, create, summarize, or rely on `.coterm/` files directly.
- If `coterm` errors, run only `coterm debug`.
