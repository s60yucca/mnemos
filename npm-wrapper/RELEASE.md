# Release Process for @s60yucca/mnemos

This document describes the process for publishing new versions of the @s60yucca/mnemos NPM wrapper package when a new mnemos binary is released.

## Overview

The @s60yucca/mnemos package version must match the mnemos binary version exactly. When a new mnemos binary is released (e.g., v1.1.11), a corresponding @s60yucca/mnemos@1.1.11 package must be published to NPM.

**Release Type**: Manual (npm publish)

## Prerequisites

- NPM account with publish access to @s60yucca/mnemos
- Node.js >=18.0.0 installed
- Git access to the mnemos repository
- NPM authentication configured (`npm login`)

## Release Checklist

### 1. Monitor mnemos Binary Releases

Watch for new releases at: https://github.com/s60yucca/mnemos/releases

When a new release is published (e.g., v1.1.11), proceed with the following steps.

### 2. Verify Binary Availability

Before publishing the NPM package, verify that all platform binaries are available:

```bash
# Check that the release includes all required binaries:
# - mnemos_darwin_amd64
# - mnemos_darwin_arm64
# - mnemos_linux_amd64
# - mnemos_linux_arm64
# - mnemos_windows_amd64.exe
# - checksums.txt

# Visit the release page and confirm all assets are present
open https://github.com/s60yucca/mnemos/releases/tag/v1.1.11
```

**Required assets:**
- ✅ mnemos_darwin_amd64
- ✅ mnemos_darwin_arm64
- ✅ mnemos_linux_amd64
- ✅ mnemos_linux_arm64
- ✅ mnemos_windows_amd64.exe
- ✅ checksums.txt

If any binaries are missing, wait for the release to be completed before proceeding.

### 3. Update Package Version

Update the package.json version to match the mnemos binary release:

```bash
cd npm-wrapper

# Update version (remove 'v' prefix from release tag)
# Example: v1.1.11 -> 1.1.11
npm version 1.1.11 --no-git-tag-version
```

**Version Alignment Rule**: The package.json version must exactly match the mnemos release tag (without the 'v' prefix).

Example:
- mnemos release: `v1.1.11`
- package.json version: `"1.1.11"`

### 4. Test Against New Binary

Before publishing, test that the wrapper correctly downloads and executes the new binary:

```bash
# Clear cache to force fresh download
rm -rf ~/.mnemos/bin

# Test version command
node cli.js --version

# Verify it downloaded the correct version
# Expected output: mnemos version 1.1.11

# Test serve command (Ctrl+C to stop)
node cli.js serve

# Test with npx (simulates user experience)
npx . --version
npx . serve
```

**Critical Tests:**
1. ✅ Binary downloads successfully
2. ✅ Checksum verification passes
3. ✅ Version output matches expected version
4. ✅ Serve command starts without errors
5. ✅ MCP stdio mode works (no stdout pollution during download)

### 5. Run Test Suite

Ensure all tests pass with the new version:

```bash
# Run unit tests
npm test

# Run linter
npm run lint

# Verify no formatting issues
npm run format
```

All tests must pass before publishing.

### 6. Verify MCP Stdio Compatibility

This is the most critical test - verify that download progress doesn't pollute stdout:

```bash
# Clear cache to test first-run download behavior
rm -rf ~/.mnemos/bin

# Test MCP stdio mode - should output only JSON-RPC, no wrapper messages on stdout
echo '{"jsonrpc":"2.0","method":"tools/list","id":1}' | node cli.js serve

# Expected: JSON-RPC response on stdout
# Expected: Download progress on stderr only
# NOT expected: [mnemos-cli] messages on stdout
```

If wrapper messages appear on stdout, the MCP integration will break. Fix before publishing.

### 7. Update Documentation (if needed)

Review and update documentation if the new release includes breaking changes or new features:

```bash
# Check mnemos release notes for changes
open https://github.com/s60yucca/mnemos/releases/tag/v1.1.11

# Update README.md if needed (new commands, changed behavior, etc.)
# Update this RELEASE.md if the process changes
```

### 8. Commit Version Change

```bash
# Commit the version bump
git add package.json package-lock.json
git commit -m "chore: bump version to 1.1.11"

# Tag the release (matches package version)
git tag v1.1.11

# Push changes and tags
git push origin main
git push origin v1.1.11
```

### 9. Publish to NPM

```bash
# Ensure you're logged in to NPM
npm whoami

# If not logged in:
# npm login

# Publish the package (public access for scoped package)
npm publish --access public

# Verify publication
npm view @s60yucca/mnemos version
# Expected output: 1.1.11
```

### 10. Verify Published Package

Test the published package to ensure it works correctly:

```bash
# Clear local cache
rm -rf ~/.mnemos/bin

# Test with npx (uses published package)
npx @s60yucca/mnemos@1.1.11 --version

# Test MCP integration
npx -y @s60yucca/mnemos@1.1.11 serve
```

### 11. Announce Release (Optional)

If appropriate, announce the new version:
- Update project documentation
- Notify users in relevant channels
- Update example configurations

## Version Alignment Verification

Before publishing, verify version alignment:

```bash
# Check package.json version
cat package.json | grep '"version"'

# Check mnemos release tag
# Visit: https://github.com/s60yucca/mnemos/releases

# Verify they match (ignoring 'v' prefix)
# package.json: "1.1.11"
# GitHub tag: v1.1.11
```

**Mismatch Resolution:**
- If package.json version is wrong: Run `npm version <correct-version> --no-git-tag-version`
- If you published the wrong version: Deprecate it with `npm deprecate @s60yucca/mnemos@<wrong-version> "Wrong version, use @s60yucca/mnemos@<correct-version>"`

## Rollback Procedure

If a published version has critical issues:

```bash
# Deprecate the broken version
npm deprecate @s60yucca/mnemos@1.1.11 "Critical bug, use @s60yucca/mnemos@1.1.10 instead"

# Publish a patch version with the fix
npm version 1.1.12 --no-git-tag-version
# ... test and publish ...
```

**Note**: NPM does not allow unpublishing versions after 24 hours. Use deprecation instead.

## Troubleshooting

### Binary Download Fails After Publishing

**Symptom**: Users report "404 Not Found" errors when downloading the binary.

**Cause**: The mnemos binary release may not be fully published yet.

**Solution**: 
1. Verify the release exists: https://github.com/s60yucca/mnemos/releases/tag/v1.1.11
2. Check that all platform binaries are present
3. Wait a few minutes for GitHub CDN propagation
4. If binaries are missing, contact the mnemos maintainer

### Checksum Verification Fails

**Symptom**: Users report checksum mismatch errors.

**Cause**: The checksums.txt file may be incorrect or corrupted.

**Solution**:
1. Download the binary manually and verify the checksum
2. Check the checksums.txt file in the GitHub release
3. If checksums are wrong, contact the mnemos maintainer to fix the release
4. Consider deprecating the NPM package version until the release is fixed

### Wrong Version Published

**Symptom**: Published @s60yucca/mnemos@1.1.11 but it downloads mnemos v1.1.10.

**Cause**: Package version doesn't match the binary version in the code.

**Solution**:
1. Deprecate the incorrect version: `npm deprecate @s60yucca/mnemos@1.1.11 "Version mismatch, use @s60yucca/mnemos@1.1.12"`
2. Fix the version in package.json
3. Publish a new patch version

### NPM Publish Permission Denied

**Symptom**: `npm publish` fails with permission error.

**Cause**: Not logged in or don't have publish access to @s60yucca/mnemos.

**Solution**:
1. Run `npm login` and authenticate
2. Verify you have publish access: `npm owner ls @s60yucca/mnemos`
3. If you don't have access, contact the package owner

## Automation Considerations

Currently, the release process is manual. Future automation options:

### Option 1: GitHub Actions Workflow

Create `.github/workflows/publish.yml` that:
1. Triggers on new mnemos release (webhook)
2. Updates package.json version automatically
3. Runs tests against new binary
4. Publishes to NPM if tests pass

**Pros**: Fully automated, fast releases  
**Cons**: Requires GitHub Actions setup, NPM token management

### Option 2: Semi-Automated Script

Create `scripts/release.sh` that:
1. Prompts for new version
2. Updates package.json
3. Runs tests
4. Publishes to NPM

**Pros**: Faster than manual, maintainer control  
**Cons**: Still requires manual trigger

### Option 3: Keep Manual (Current)

**Pros**: Full control, simple process, no automation overhead  
**Cons**: Slower releases, potential for human error

**Recommendation**: Start with manual process (current). Consider automation if release frequency increases or if manual process becomes a bottleneck.

## Release Frequency

Expected release frequency: 1-4 releases per month (following mnemos binary releases)

Typical release timeline:
1. mnemos binary released: Day 0
2. NPM package testing: Day 0-1
3. NPM package published: Day 1
4. User adoption: Day 1-7

## Contact

For questions about the release process:
- GitHub Issues: https://github.com/s60yucca/mnemos/issues
- Package Maintainer: s60yucca

## Changelog

- 2024-01: Initial release process documentation
