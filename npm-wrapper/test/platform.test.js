/**
 * Unit tests for platform detection module
 */

const { detectPlatform } = require('../lib/platform');

describe('Platform Detection', () => {
  let originalPlatform;
  let originalArch;

  beforeEach(() => {
    // Save original values
    originalPlatform = process.platform;
    originalArch = process.arch;
  });

  afterEach(() => {
    // Restore original values
    Object.defineProperty(process, 'platform', {
      value: originalPlatform,
      writable: true,
      configurable: true,
    });
    Object.defineProperty(process, 'arch', {
      value: originalArch,
      writable: true,
      configurable: true,
    });
  });

  describe('Supported Platforms', () => {
    test('detects macOS Intel (darwin-x64)', () => {
      Object.defineProperty(process, 'platform', { value: 'darwin' });
      Object.defineProperty(process, 'arch', { value: 'x64' });

      const result = detectPlatform();

      expect(result).toEqual({
        os: 'darwin',
        arch: 'amd64',
        archiveName: 'mnemos_darwin_amd64.tar.gz',
        binaryName: 'mnemos',
      });
    });

    test('detects macOS Apple Silicon (darwin-arm64)', () => {
      Object.defineProperty(process, 'platform', { value: 'darwin' });
      Object.defineProperty(process, 'arch', { value: 'arm64' });

      const result = detectPlatform();

      expect(result).toEqual({
        os: 'darwin',
        arch: 'arm64',
        archiveName: 'mnemos_darwin_arm64.tar.gz',
        binaryName: 'mnemos',
      });
    });

    test('detects Linux x64 (linux-x64)', () => {
      Object.defineProperty(process, 'platform', { value: 'linux' });
      Object.defineProperty(process, 'arch', { value: 'x64' });

      const result = detectPlatform();

      expect(result).toEqual({
        os: 'linux',
        arch: 'amd64',
        archiveName: 'mnemos_linux_amd64.tar.gz',
        binaryName: 'mnemos',
      });
    });

    test('detects Linux ARM64 (linux-arm64)', () => {
      Object.defineProperty(process, 'platform', { value: 'linux' });
      Object.defineProperty(process, 'arch', { value: 'arm64' });

      const result = detectPlatform();

      expect(result).toEqual({
        os: 'linux',
        arch: 'arm64',
        archiveName: 'mnemos_linux_arm64.tar.gz',
        binaryName: 'mnemos',
      });
    });

    test('detects Windows x64 (win32-x64)', () => {
      Object.defineProperty(process, 'platform', { value: 'win32' });
      Object.defineProperty(process, 'arch', { value: 'x64' });

      const result = detectPlatform();

      expect(result).toEqual({
        os: 'windows',
        arch: 'amd64',
        archiveName: 'mnemos_windows_amd64.zip',
        binaryName: 'mnemos.exe',
      });
    });
  });

  describe('Unsupported Platforms', () => {
    test('returns error for unsupported platform (freebsd)', () => {
      Object.defineProperty(process, 'platform', { value: 'freebsd' });
      Object.defineProperty(process, 'arch', { value: 'x64' });

      const result = detectPlatform();

      expect(result).toBeInstanceOf(Error);
      expect(result.message).toContain('Unsupported platform: freebsd');
      expect(result.message).toContain('Supported platforms:');
      expect(result.message).toContain('darwin-x64');
      expect(result.message).toContain('darwin-arm64');
      expect(result.message).toContain('linux-x64');
      expect(result.message).toContain('linux-arm64');
      expect(result.message).toContain('win32-x64');
    });

    test('returns error for unsupported architecture on macOS (darwin-ia32)', () => {
      Object.defineProperty(process, 'platform', { value: 'darwin' });
      Object.defineProperty(process, 'arch', { value: 'ia32' });

      const result = detectPlatform();

      expect(result).toBeInstanceOf(Error);
      expect(result.message).toContain('Unsupported architecture: darwin-ia32');
      expect(result.message).toContain('Supported platforms:');
    });

    test('returns error for unsupported architecture on Linux (linux-arm)', () => {
      Object.defineProperty(process, 'platform', { value: 'linux' });
      Object.defineProperty(process, 'arch', { value: 'arm' });

      const result = detectPlatform();

      expect(result).toBeInstanceOf(Error);
      expect(result.message).toContain('Unsupported architecture: linux-arm');
      expect(result.message).toContain('Supported platforms:');
    });

    test('returns error for unsupported architecture on Windows (win32-arm64)', () => {
      Object.defineProperty(process, 'platform', { value: 'win32' });
      Object.defineProperty(process, 'arch', { value: 'arm64' });

      const result = detectPlatform();

      expect(result).toBeInstanceOf(Error);
      expect(result.message).toContain('Unsupported architecture: win32-arm64');
      expect(result.message).toContain('Supported platforms:');
    });

    test('returns error for completely unsupported platform (aix)', () => {
      Object.defineProperty(process, 'platform', { value: 'aix' });
      Object.defineProperty(process, 'arch', { value: 'ppc64' });

      const result = detectPlatform();

      expect(result).toBeInstanceOf(Error);
      expect(result.message).toContain('Unsupported platform: aix');
    });
  });

  describe('Error Message Quality', () => {
    test('error message includes GitHub link', () => {
      Object.defineProperty(process, 'platform', { value: 'sunos' });
      Object.defineProperty(process, 'arch', { value: 'x64' });

      const result = detectPlatform();

      expect(result).toBeInstanceOf(Error);
      expect(result.message).toContain('https://github.com/s60yucca/mnemos#installation');
    });

    test('error message lists all supported combinations', () => {
      Object.defineProperty(process, 'platform', { value: 'openbsd' });
      Object.defineProperty(process, 'arch', { value: 'x64' });

      const result = detectPlatform();

      expect(result).toBeInstanceOf(Error);
      const message = result.message;
      
      // Verify all 5 supported combinations are listed
      expect(message).toContain('darwin-x64');
      expect(message).toContain('darwin-arm64');
      expect(message).toContain('linux-x64');
      expect(message).toContain('linux-arm64');
      expect(message).toContain('win32-x64');
    });
  });

  describe('Return Value Structure', () => {
    test('returns object with correct properties for supported platform', () => {
      Object.defineProperty(process, 'platform', { value: 'darwin' });
      Object.defineProperty(process, 'arch', { value: 'arm64' });

      const result = detectPlatform();

      expect(result).toHaveProperty('os');
      expect(result).toHaveProperty('arch');
      expect(result).toHaveProperty('archiveName');
      expect(result).toHaveProperty('binaryName');
      expect(typeof result.os).toBe('string');
      expect(typeof result.arch).toBe('string');
      expect(typeof result.archiveName).toBe('string');
      expect(typeof result.binaryName).toBe('string');
    });

    test('Windows binary name includes .exe extension', () => {
      Object.defineProperty(process, 'platform', { value: 'win32' });
      Object.defineProperty(process, 'arch', { value: 'x64' });

      const result = detectPlatform();

      expect(result.binaryName).toBe('mnemos.exe');
      expect(result.binaryName).toMatch(/\.exe$/);
    });

    test('Unix binary names do not include extension', () => {
      Object.defineProperty(process, 'platform', { value: 'linux' });
      Object.defineProperty(process, 'arch', { value: 'x64' });

      const result = detectPlatform();

      expect(result.binaryName).toBe('mnemos');
      expect(result.binaryName).not.toMatch(/\./);
    });

    test('archive names use correct format with underscores', () => {
      Object.defineProperty(process, 'platform', { value: 'linux' });
      Object.defineProperty(process, 'arch', { value: 'arm64' });

      const result = detectPlatform();

      expect(result.archiveName).toBe('mnemos_linux_arm64.tar.gz');
      expect(result.archiveName).toMatch(/^mnemos_\w+_\w+\.(tar\.gz|zip)$/);
    });

    test('Windows uses .zip archive format', () => {
      Object.defineProperty(process, 'platform', { value: 'win32' });
      Object.defineProperty(process, 'arch', { value: 'x64' });

      const result = detectPlatform();

      expect(result.archiveName).toMatch(/\.zip$/);
    });

    test('Unix platforms use .tar.gz archive format', () => {
      Object.defineProperty(process, 'platform', { value: 'darwin' });
      Object.defineProperty(process, 'arch', { value: 'x64' });

      const result = detectPlatform();

      expect(result.archiveName).toMatch(/\.tar\.gz$/);
    });
  });
});
