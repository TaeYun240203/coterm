# coterm

`coterm` runs agent task commands in a user-visible shared tmux terminal.

## Install

```bash
curl -fsSL https://raw.githubusercontent.com/<owner>/coterm/main/scripts/install.sh | sh
```

## Open A Workspace Terminal

```bash
coterm open
```

## Run A Command

```bash
coterm run --pane main1 -- npm test
```
