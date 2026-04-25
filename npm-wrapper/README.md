# @s60yucca/mnemos

Cross-platform NPM wrapper for the [mnemos](https://github.com/s60yucca/mnemos) autopilot knowledge base system.

## Quick Start

Get started with mnemos in seconds using npx:

```bash
# Start the MCP server (most common use case)
npx -y @s60yucca/mnemos serve

# Check version
npx @s60yucca/mnemos --version

# Store your first memory
npx @s60yucca/mnemos store --content "My first memory" --type long_term
```

No installation, configuration, or PATH setup required!

## Installation

### Using npx (Recommended)

No installation required! Run mnemos directly:

```bash
npx @s60yucca/mnemos --version
npx @s60yucca/mnemos serve
```

### Global Installation

```bash
npm install -g @s60yucca/mnemos
mnemos --version
mnemos serve
```

### Local Installation

```bash
npm install @s60yucca/mnemos
npx mnemos serve
```

## Usage

### MCP Integration (Primary Use Case)

The main use case for this package is integrating mnemos with MCP-compatible AI tools.

#### Kiro Configuration

Add this to your Kiro MCP configuration (`.kiro/settings/mcp.json`):

```json
{
  "mcpServers": {
    "mnemos": {
      "command": "npx",
      "args": ["-y", "@s60yucca/mnemos", "serve"]
    }
  }
}
```

#### Claude Desktop Configuration

Add this to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json` on macOS):

```json
{
  "mcpServers": {
    "mnemos": {
      "command": "npx",
      "args": ["-y", "@s60yucca/mnemos", "serve"]
    }
  }
}
```

#### Other MCP Clients

The configuration pattern is the same for any MCP-compatible client:
- **command**: `"npx"`
- **args**: `["-y", "@s60yucca/mnemos", "serve"]`

The `-y` flag skips confirmation prompts for immediate execution.

### Basic Commands

```bash
# Check version
npx @s60yucca/mnemos --version

# Start MCP server
npx @s60yucca/mnemos serve

# Store a memory
npx @s60yucca/mnemos store --content "Important fact" --type long_term

# Search memories
npx @s60yucca/mnemos search --query "important"

# Get context for a query
npx @s60yucca/mnemos context --query "authentication" --max-tokens 2000

# List all memories
npx @s60yucca/mnemos list

# Delete a memory
npx @s60yucca/mnemos delete --id <memory-id>
```

### Advanced Usage

```bash
# Store with tags and category
npx @s60yucca/mnemos store \
  --content "API uses JWT tokens" \
  --type semantic \
  --category architecture \
  --tags "auth,jwt,api"

# Search with filters
npx @s60yucca/mnemos search \
  --query "authentication" \
  --mode hybrid \
  --limit 5

# Run maintenance (decay, archival, GC)
npx @s60yucca/mnemos maintain
```

## How It Works

This package automatically:

1. **Detects your platform** - Identifies your OS (macOS, Linux, Windows) and architecture (x64, ARM64)
2. **Downloads the binary** - Fetches the appropriate mnemos binary from [GitHub releases](https://github.com/s60yucca/mnemos/releases)
3. **Verifies integrity** - Checks SHA256 checksums to ensure the download is not corrupted
4. **Caches locally** - Stores the binary in `~/.mnemos/bin/` for fast subsequent executions
5. **Forwards commands** - Passes all arguments and stdio streams transparently to the mnemos binary

First run: 5-30 seconds (downloads binary)  
Subsequent runs: <100ms overhead (uses cached binary)

## Supported Platforms

| Platform | Architecture | Binary Name |
|----------|-------------|-------------|
| macOS | Intel (x64) | ✅ mnemos_darwin_amd64 |
| macOS | Apple Silicon (ARM64) | ✅ mnemos_darwin_arm64 |
| Linux | x64 | ✅ mnemos_linux_amd64 |
| Linux | ARM64 | ✅ mnemos_linux_arm64 |
| Windows | x64 | ✅ mnemos_windows_amd64.exe |
| Windows | ARM64 | ❌ Not supported |
| Linux | ARM 32-bit | ❌ Not supported |

All binaries are built and distributed via [GoReleaser](https://goreleaser.com/) from the official mnemos repository.

## Troubleshooting

### Download Issues

If the binary download fails:

```bash
# Check your internet connection
# Verify the release exists: https://github.com/s60yucca/mnemos/releases

# Clear the cache and retry
rm -rf ~/.mnemos/bin
npx @s60yucca/mnemos --version
```

**Common causes:**
- Network connectivity issues
- GitHub rate limiting (wait a few minutes and retry)
- Release not yet published for your platform
- Firewall or proxy blocking GitHub downloads

### Permission Issues

On Unix systems, if you get permission errors:

```bash
# Fix executable permissions
chmod +x ~/.mnemos/bin/v*/mnemos*

# Or clear cache and re-download (will set permissions automatically)
rm -rf ~/.mnemos/bin
npx @s60yucca/mnemos --version
```

### Platform Not Supported

If you see "Unsupported platform" error, check that you're using one of the supported platforms:

- **macOS**: Intel (x64) and Apple Silicon (ARM64)
- **Linux**: x64 and ARM64
- **Windows**: x64

**Note**: Windows ARM64 and 32-bit architectures are not currently supported.

### MCP Server Not Responding

If the MCP server doesn't respond in your AI tool:

1. Test the server manually:
   ```bash
   npx @s60yucca/mnemos serve
   ```

2. Check that the binary downloaded successfully:
   ```bash
   ls -la ~/.mnemos/bin/
   ```

3. Verify your MCP configuration syntax (must be valid JSON)

4. Check the AI tool's logs for error messages

### Slow First Run

The first execution downloads the binary (5-10 MB), which may take 5-30 seconds depending on your connection. Subsequent runs use the cached binary and start instantly.

To pre-download the binary:
```bash
npx @s60yucca/mnemos --version
```

### Checksum Verification Failed

If you see a checksum mismatch error:

```bash
# Clear the corrupted download
rm -rf ~/.mnemos/bin

# Retry the download
npx @s60yucca/mnemos --version
```

If the error persists, the GitHub release may be corrupted. Check the [releases page](https://github.com/s60yucca/mnemos/releases) or file an issue.

## Version Alignment

The NPM package version matches the mnemos binary version exactly. For example:

- `@s60yucca/mnemos@1.1.10` downloads mnemos `v1.1.10`
- `@s60yucca/mnemos@1.1.9` downloads mnemos `v1.1.9`

## Development

See the [main mnemos repository](https://github.com/s60yucca/mnemos) for development instructions.

## License

MIT - See [LICENSE](LICENSE) file for details.
