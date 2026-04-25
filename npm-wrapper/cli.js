#!/usr/bin/env node

/**
 * Main CLI entry point for @s60yucca/mnemos
 * Coordinates platform detection, binary caching, downloading, and execution
 */

const { detectPlatform } = require('./lib/platform');
const { ensureCached } = require('./lib/cache');
const { downloadBinary } = require('./lib/download');
const { executeBinary } = require('./lib/execute');
const packageJson = require('./package.json');

/**
 * Main execution function
 */
async function main() {
  try {
    // Detect platform
    const platformInfo = detectPlatform();
    
    // Check if platform detection returned an error
    if (platformInfo instanceof Error) {
      process.stderr.write(`[mnemos-cli] Error: ${platformInfo.message}\n`);
      process.exit(1);
    }
    
    // Get version from package.json
    const version = packageJson.version;
    
    // Ensure binary is cached (download if needed)
    const binaryPath = await ensureCached(version, platformInfo, downloadBinary);
    
    // Get command-line arguments (skip node and script name)
    const args = process.argv.slice(2);
    
    // Execute binary with forwarded arguments
    const exitCode = await executeBinary(binaryPath, args);
    
    // Exit with the same code as the binary
    process.exit(exitCode);
    
  } catch (err) {
    // Handle all errors with descriptive messages to stderr
    process.stderr.write(`[mnemos-cli] Error: ${err.message}\n`);
    process.exit(1);
  }
}

// Run main function
main();
