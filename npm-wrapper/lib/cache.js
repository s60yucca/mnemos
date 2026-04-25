/**
 * Binary cache management module
 * Handles caching of downloaded mnemos binaries in ~/.mnemos/bin/
 */

const fs = require('fs');
const path = require('path');
const os = require('os');

/**
 * Get the cache directory path for a specific version
 * @param {string} version - Package version (e.g., '1.1.10')
 * @returns {string} Cache directory path (~/.mnemos/bin/v{version}/)
 */
function getCachePath(version) {
  const homeDir = os.homedir();
  const versionTag = version.startsWith('v') ? version : `v${version}`;
  return path.join(homeDir, '.mnemos', 'bin', versionTag);
}

/**
 * Check if a binary is already cached
 * @param {string} version - Package version
 * @param {string} binaryName - Binary filename (e.g., 'mnemos' or 'mnemos.exe')
 * @returns {boolean} True if binary exists in cache
 */
function isCached(version, binaryName) {
  const cachePath = getCachePath(version);
  const binaryPath = path.join(cachePath, binaryName);
  
  try {
    return fs.existsSync(binaryPath);
  } catch (err) {
    return false;
  }
}

/**
 * Ensure binary is cached, downloading if necessary
 * @param {string} version - Package version
 * @param {Object} platformInfo - Platform information from detectPlatform()
 * @param {Function} downloadFn - Download function to call if not cached
 * @returns {Promise<string>} Path to cached binary
 */
async function ensureCached(version, platformInfo, downloadFn) {
  const cachePath = getCachePath(version);
  const binaryPath = path.join(cachePath, platformInfo.binaryName);
  
  // Check if already cached
  if (isCached(version, platformInfo.binaryName)) {
    return binaryPath;
  }
  
  // Create cache directory if it doesn't exist
  try {
    fs.mkdirSync(cachePath, { recursive: true });
  } catch (err) {
    throw new Error(
      `Failed to create cache directory: ${cachePath}\n` +
      `Error: ${err.message}\n` +
      `Action: Check disk space and permissions`
    );
  }
  
  // Download binary
  await downloadFn(version, platformInfo, cachePath);
  
  return binaryPath;
}

module.exports = {
  getCachePath,
  isCached,
  ensureCached,
};
