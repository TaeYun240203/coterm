package cli

const HelpText = `coterm

Usage:
  coterm open
  coterm run [--client <id>] --pane <name> -- <command> ...
  coterm run [--client <id>] --pane <name> --stdin
  coterm run [--client <id>] --pane <name> --detach -- <command> ...
  coterm sync [--client <id>]
  coterm read [--client <id>] --pane <name>
  coterm snapshot [--client <id>] --pane <name> [--lines <n>] [--bytes <n>]
  coterm panes
  coterm pane close --pane <name>
  coterm export --format jsonl
  coterm uninstall [--purge]
  coterm debug
`
