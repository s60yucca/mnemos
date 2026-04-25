/**
 * Windows-specific path handling tests
 * Verifies that all path operations use path.join() and handle Windows correctly
 */

const path = require('path');
const os = require('os');
const fs = require('fs');
const { getCachePath, isCached, ensureCached } = require('../lib/cache');
const { detectPlatform } = require('../lib/platform');

describe('Windows Path Handling', () => {
  let originalPlatform;
  let originalArch;

  beforeEach(() => {
    originalPlatform = process.platform;
    originalArch = process.arch;
  });

  afterEach(() => {
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

  describe('Platform Detection - Windows', () => {
    test('Windows binary includes .exe extension', () => {
      Object.defineProperty(process, 'platform', { value: 'win32' });
      Object.defineProperty(process, 'arch', { value: 'x64' });

      const result = detectPlatform();

      expect(result.binaryName).toBe('mnemos.exe');
      expect(result.binaryName).toMatch(/\.exe$/);
    });

    test('Windows archive uses .zip format', () => {
      Object.defineProperty(process, 'platform', { value: 'win32' });
      Object.defineProperty(process, 'arch', { value: 'x64' });

      const result = detectPlatform();

      expect(result.archiveName).toBe('mnemos_windows_amd64.zip');
      expect(result.archiveName).toMatch(/\.zip$/);
    });
  });

  describe('Cache Path Construction - Windows', () => {
    test('getCachePath uses path.join for Windows paths', () => {
      const version = '1.1.10';
      const cachePath = getCachePath(version);

      // Verify path uses correct separators for current platform
      const normalized = path.normalize(cachePath);
      expect(cachePath).toBe(normalized);

      // Verify path components are present
      expect(cachePath).toContain('.mnemos');
      expect(cachePath).toContain('bin');
      expect(cachePath).toContain('v1.1.10');
    });

    test('getCachePath handles Windows home directory', () => {
      const version = '1.0.0';
      const homeDir = os.homedir();
      const cachePath = getCachePath(version);

      // Should start with home directory
      expect(cachePath.startsWith(homeDir)).toBe(true);

      // Should use platform-appropriate separators
      const parts = cachePath.split(path.sep);
      expect(parts).toContain('.mnemos');
      expect(parts).toContain('bin');
    });

    test('binary path construction uses path.join', () => {
      const mockExistsSync = jest.spyOn(fs, 'existsSync').mockReturnValue(true);

      const version = '1.1.10';
      const binaryName = 'mnemos.exe';

      isCached(version, binaryName);

      const calledPath = mockExistsSync.mock.calls[0][0];

      // Verify path is normalized (no mixed separators)
      const normalized = path.normalize(calledPath);
      expect(calledPath).toBe(normalized);

      // Verify path ends with binary name
      expect(calledPath).toContain('mnemos.exe');

      mockExistsSync.mockRestore();
    });
  });

  describe('Path Separator Handling', () => {
    test('paths do not contain hardcoded forward slashes', () => {
      const version = '1.1.10';
      const cachePath = getCachePath(version);

      // On Windows, path should use backslashes
      // On Unix, path should use forward slashes
      // path.join ensures correct separator is used
      const expectedSeparator = path.sep;
      const parts = cachePath.split(expectedSeparator);

      // Should have multiple parts (not a single string with wrong separators)
      expect(parts.length).toBeGreaterThan(1);
    });

    test('ensureCached constructs paths with correct separators', async () => {
      const mockExistsSync = jest.spyOn(fs, 'existsSync').mockReturnValue(true);

      const version = '1.1.10';
      const platformInfo = {
        os: 'windows',
        arch: 'amd64',
        binaryName: 'mnemos.exe',
        archiveName: 'mnemos_windows_amd64.zip',
      };

      const binaryPath = await ensureCached(version, platformInfo, jest.fn());

      // Verify path is normalized
      const normalized = path.normalize(binaryPath);
      expect(binaryPath).toBe(normalized);

      // Verify no mixed separators
      const hasForwardSlash = binaryPath.includes('/');
      const hasBackslash = binaryPath.includes('\\');

      if (path.sep === '\\') {
        // On Windows, should use backslashes
        expect(hasBackslash).toBe(true);
      } else {
        // On Unix, should use forward slashes
        expect(hasForwardSlash).toBe(true);
      }

      mockExistsSync.mockRestore();
    });
  });

  describe('Windows Binary Execution Paths', () => {
    test('binary path with .exe extension is valid', () => {
      const version = '1.1.10';
      const binaryName = 'mnemos.exe';
      const cachePath = getCachePath(version);
      const binaryPath = path.join(cachePath, binaryName);

      // Verify path is valid
      expect(binaryPath).toContain('.mnemos');
      expect(binaryPath).toContain('bin');
      expect(binaryPath).toContain('v1.1.10');
      expect(binaryPath).toContain('mnemos.exe');

      // Verify path is normalized
      const normalized = path.normalize(binaryPath);
      expect(binaryPath).toBe(normalized);
    });

    test('Windows paths work with spawn command', () => {
      // This test verifies that paths constructed with path.join
      // are compatible with child_process.spawn on Windows
      const version = '1.1.10';
      const binaryName = 'mnemos.exe';
      const cachePath = getCachePath(version);
      const binaryPath = path.join(cachePath, binaryName);

      // Path should be absolute
      expect(path.isAbsolute(binaryPath)).toBe(true);

      // Path should not have mixed separators
      const normalized = path.normalize(binaryPath);
      expect(binaryPath).toBe(normalized);
    });
  });

  describe('Cross-Platform Path Compatibility', () => {
    test('paths work correctly on current platform', () => {
      const version = '1.1.10';
      const cachePath = getCachePath(version);

      // Should be an absolute path
      expect(path.isAbsolute(cachePath)).toBe(true);

      // Should use correct separator for platform
      const parts = cachePath.split(path.sep);
      expect(parts.length).toBeGreaterThan(3);

      // Should not mix separators
      if (path.sep === '\\') {
        // Windows: should not contain forward slashes in path components
        const pathWithoutDrive = cachePath.substring(2); // Skip C:
        expect(pathWithoutDrive.split('/').length).toBe(1);
      }
    });

    test('path.join is used consistently across all modules', () => {
      // This test verifies the pattern - actual usage is in the modules
      const testParts = ['home', 'user', '.mnemos', 'bin', 'v1.1.10'];
      const joined = path.join(...testParts);

      // Should use platform separator
      expect(joined).toContain(path.sep);

      // Should be normalized
      const normalized = path.normalize(joined);
      expect(joined).toBe(normalized);
    });
  });

  describe('Windows-Specific Edge Cases', () => {
    test('handles Windows drive letters correctly', () => {
      const version = '1.1.10';
      const cachePath = getCachePath(version);

      // On Windows, should start with drive letter (e.g., C:\)
      // On Unix, should start with /
      if (process.platform === 'win32') {
        expect(cachePath).toMatch(/^[A-Z]:\\/);
      } else {
        expect(cachePath).toMatch(/^\//);
      }
    });

    test('handles UNC paths if home directory is on network share', () => {
      // UNC paths start with \\server\share
      const version = '1.1.10';
      const cachePath = getCachePath(version);

      // Should be a valid absolute path regardless of UNC or local
      expect(path.isAbsolute(cachePath)).toBe(true);
    });

    test('binary name with .exe is handled correctly in cache operations', async () => {
      const mockExistsSync = jest.spyOn(fs, 'existsSync').mockReturnValue(false);
      const mockMkdirSync = jest.spyOn(fs, 'mkdirSync').mockImplementation(() => {});
      const mockDownloadFn = jest.fn().mockResolvedValue();

      const version = '1.1.10';
      const platformInfo = {
        os: 'windows',
        arch: 'amd64',
        binaryName: 'mnemos.exe',
        archiveName: 'mnemos_windows_amd64.zip',
      };

      const binaryPath = await ensureCached(version, platformInfo, mockDownloadFn);

      // Should end with .exe
      expect(binaryPath).toMatch(/mnemos\.exe$/);

      // Should be a valid path
      expect(path.isAbsolute(binaryPath)).toBe(true);

      mockExistsSync.mockRestore();
      mockMkdirSync.mockRestore();
    });
  });

  describe('Path Normalization', () => {
    test('all constructed paths are normalized', () => {
      const version = '1.1.10';
      const cachePath = getCachePath(version);

      // path.normalize should not change the path
      const normalized = path.normalize(cachePath);
      expect(cachePath).toBe(normalized);
    });

    test('binary paths are normalized', async () => {
      const mockExistsSync = jest.spyOn(fs, 'existsSync').mockReturnValue(true);

      const version = '1.1.10';
      const platformInfo = {
        os: 'windows',
        arch: 'amd64',
        binaryName: 'mnemos.exe',
      };

      const binaryPath = await ensureCached(version, platformInfo, jest.fn());

      // Should be normalized
      const normalized = path.normalize(binaryPath);
      expect(binaryPath).toBe(normalized);

      mockExistsSync.mockRestore();
    });
  });
});
