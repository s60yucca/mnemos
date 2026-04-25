/**
 * Platform detection module for mnemos binary wrapper
 * Maps Node.js platform/arch to mnemos binary naming conventions
 */

/**
 * Detects the current platform and returns binary naming information
 * @returns {Object|Error} Platform info with archiveName and binaryName, or Error for unsupported platforms
 */
function detectPlatform() {
  const platform = process.platform;
  const arch = process.arch;

  // Platform mapping based on BINARY_VERIFICATION.md
  const platformMap = {
    darwin: {
      x64: {
        os: 'darwin',
        arch: 'amd64',
        archiveName: 'mnemos_darwin_amd64.tar.gz',
        binaryName: 'mnemos',
      },
      arm64: {
        os: 'darwin',
        arch: 'arm64',
        archiveName: 'mnemos_darwin_arm64.tar.gz',
        binaryName: 'mnemos',
      },
    },
    linux: {
      x64: {
        os: 'linux',
        arch: 'amd64',
        archiveName: 'mnemos_linux_amd64.tar.gz',
        binaryName: 'mnemos',
      },
      arm64: {
        os: 'linux',
        arch: 'arm64',
        archiveName: 'mnemos_linux_arm64.tar.gz',
        binaryName: 'mnemos',
      },
    },
    win32: {
      x64: {
        os: 'windows',
        arch: 'amd64',
        archiveName: 'mnemos_windows_amd64.zip',
        binaryName: 'mnemos.exe',
      },
    },
  };

  // Check if platform is supported
  if (!platformMap[platform]) {
    return new Error(
      `Unsupported platform: ${platform}\n\n` +
        `Supported platforms:\n` +
        `  - macOS (Intel): darwin-x64\n` +
        `  - macOS (Apple Silicon): darwin-arm64\n` +
        `  - Linux (x64): linux-x64\n` +
        `  - Linux (ARM64): linux-arm64\n` +
        `  - Windows (x64): win32-x64\n\n` +
        `For more information: https://github.com/s60yucca/mnemos#installation`
    );
  }

  // Check if architecture is supported for this platform
  if (!platformMap[platform][arch]) {
    return new Error(
      `Unsupported architecture: ${platform}-${arch}\n\n` +
        `Supported platforms:\n` +
        `  - macOS (Intel): darwin-x64\n` +
        `  - macOS (Apple Silicon): darwin-arm64\n` +
        `  - Linux (x64): linux-x64\n` +
        `  - Linux (ARM64): linux-arm64\n` +
        `  - Windows (x64): win32-x64\n\n` +
        `For more information: https://github.com/s60yucca/mnemos#installation`
    );
  }

  return platformMap[platform][arch];
}

module.exports = { detectPlatform };
