# signoz-cli

> **Unofficial, community-built, not affiliated with SigNoz**

`signoz` is a small Go CLI for the [SigNoz](https://signoz.io/) REST API. It targets humans and AI agents that want structured JSON (or tables) instead of wiring an MCP server — see [SigNoz/signoz#11828](https://github.com/SigNoz/signoz/issues/11828).

MIT licensed. Third-party notices in [NOTICE](NOTICE); release archives also ship `THIRD_PARTY_NOTICES` and `third-party-licenses/`.

## Install

Download a release archive from [GitHub Releases](https://github.com/swtormy-agents/signoz-cli/releases) (Linux and Windows, amd64/arm64). Extract the binary and add it to your `PATH`.

Or install from source with Go 1.23+:

```bash
go install github.com/swtormy-agents/signoz-cli/cmd/signoz@feat/v0.1
```

## Quick start

```bash
export SIGNOZ_URL=https://your-instance.signoz.cloud
export SIGNOZ_API_KEY=...    # service account key — never pass on the command line

signoz health
signoz logs query --filter "severity_text = 'ERROR'" --limit 20
signoz traces query --filter "service.name = 'frontend'" --limit 20
signoz alerts list --limit 20
signoz services list
```

Pipe the API key instead of exporting it:

```bash
echo "$SIGNOZ_API_KEY" | signoz health
```

## Output

- Default: JSON on stdout; one JSON error object on stderr
- `--output table` for simple tabular views
- `--jq '...'` to filter JSON (gojq)
- `--pretty` to indent JSON

## Safety

- Never log or print the API key; `--dry-run` and errors redact it as `<API_KEY>`
- Agents stay read-only. Mutating `signoz api` calls need `--write`, a TTY, and typing `write` when prompted
- Exit codes: `0` ok, `1` runtime/API, `2` usage or config, `3` auth, `4` blocked

See [docs/api-scope.md](docs/api-scope.md) for endpoints covered in v0.1.

## Agent skills

Bundled skill (also in [skills/signoz/SKILL.md](skills/signoz/SKILL.md)):

```bash
signoz skill install --agent cursor
signoz skill print
```

## License

MIT — see [LICENSE](LICENSE).
