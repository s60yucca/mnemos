#!/usr/bin/env bash
set -e

echo "Fixing Homebrew dual Formula/Cask issue..."

# Step 1: Uninstall the old formula version
echo "1. Uninstalling old formula version..."
brew uninstall s60yucca/tap/mnemos 2>/dev/null || true

# Step 2: Clear tap cache
echo "2. Clearing tap cache..."
brew untap s60yucca/tap 2>/dev/null || true

# Step 3: Re-add tap
echo "3. Re-adding tap..."
brew tap s60yucca/tap

# Step 4: Install the CASK (not formula)
echo "4. Installing cask version..."
brew install --cask s60yucca/tap/mnemos

# Step 5: Verify
echo "5. Verifying installation..."
mnemos --version

echo ""
echo "Done! You should now have version 1.0.9"
echo ""
echo "IMPORTANT: You need to delete Formula/mnemos.rb from your tap repository"
echo "to prevent this issue in future releases."
