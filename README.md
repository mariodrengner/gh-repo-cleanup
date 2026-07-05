# gh-repo-cleanup

Clean up your GitHub portfolio — safely.

Old forks you never touched again, repos that stopped being maintained years
ago: `gh repo-cleanup` finds them, explains *why* each one is a candidate,
and archives or deletes them through an interactive picker. Every deletion
writes a local `git bundle` backup first — **no backup, no delete.**

## Install

```bash
gh extension install mariodrengner/gh-repo-cleanup
```

Requires the [GitHub CLI](https://cli.github.com) (authenticated) and `git`.

## Usage

```bash
gh repo-cleanup                 # interactive: scan → mark → confirm → execute
gh repo-cleanup --list          # read-only candidate report
gh repo-cleanup --json          # read-only, machine-readable
gh repo-cleanup --older-than 6m # custom inactivity threshold (90d, 6m, 1y)
```

Deleting repos needs the `delete_repo` scope:

```bash
gh auth refresh -s delete_repo
```

## What counts as a candidate?

| Candidate | Suggested action |
|---|---|
| Fork with no own commits ahead of upstream | delete |
| Fork inactive longer than the threshold (own commits are warned about) | delete |
| Own repo with no push for longer than the threshold | archive |

Protections: forks with an open PR against upstream are never suggested;
stars, forks and open issues are shown as warnings; archived repos are
skipped; nothing is preselected.

## Safety

- Deletions must be confirmed by literally typing `delete <N>`.
- Before every deletion: mirror clone → `git bundle` (all branches + tags)
  + `metadata.json` into `~/.local/share/gh-repo-cleanup/backups/`.
  If the backup fails, the repo is **not** deleted.
- Restore: `git clone <bundle> <name> && cd <name> && gh repo create <name> --private --source=. --push`
- Archiving is reversible: `gh repo unarchive <owner>/<repo>`

## License

MIT — sibling project of [dev-cleanup](https://github.com/mariodrengner/dev-cleanup).
