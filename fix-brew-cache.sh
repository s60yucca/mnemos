#!/usr/bin/env bash
# Fix Homebrew cache issue - force update to latest version

echo "Fixing Homebrew cache for mnemos..."

# Step 1: Untap and retap to clear cache
echo "1. Clearing tap cache..."
brew untap s60yucca/tap 2>/dev/null || true
brew tap s60yucca/tap

# Step 2: Update Homebrew
echo "2. Updating Homebrew..."
brew update

# Step 3: Reinstall (not upgrade, since cache is stale)
echo "3. Reinstalling mnemos..."
brew uninstall s60yucca/tap/mnemos 2>/dev/null || true
brew install s60yucca/tap/mnemos --force

# Step 4: Verify version
echo "4. Verifying installation..."
mnemos --version

echo "Done! You should now have version 1.0.9"
