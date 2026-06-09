/**
 * Unit tests for cache management module
 */

const fs = require('fs');
const path = require('path');
const os = require('os');
const { getCachePath, isCached, ensureCached } = require('../lib/cache');

describe('Cache Management', () => {
  const testVersion = '1.1.10';
  const testBinaryName = 'mnemos';
  const testBinaryNameWindows = 'mnemos.exe';

  describe('getCachePath', () => {
    test('returns correct cache path with version', () => {
      const cachePath = getCachePath(testVersion);
      const homeDir = os.homedir();
      const expected = path.join(homeDir, '.mnemos', 'bin', 'v1.1.10');

      expect(cachePath).toBe(expected);
    });

    test('adds v prefix if not present', () => {
      const cachePath = getCachePath('1.2.3');
      expect(cachePath).toContain('v1.2.3');
    });

    test('does not double v prefix', () => {
      const cachePath = getCachePath('v1.2.3');
      expect(cachePath).toContain('v1.2.3');
      expect(cachePath).not.toContain('vv1.2.3');
    });

    test('uses correct path separators for platform', () => {
      const cachePath = getCachePath(testVersion);
      const homeDir = os.homedir();

      // Path should start with home directory
      expect(cachePath).toContain(homeDir);
      
      // Path should contain .mnemos/bin structure
      expect(cachePath).toContain('.mnemos');
      expect(cachePath).toContain('bin');
    });

    test('handles different version formats', () => {
      const versions = ['0.1.0', 'v0.1.0', '1.0.0-beta', 'v2.3.4-rc1'];
      
      versions.forEach(version => {
        const cachePath = getCachePath(version);
        expect(cachePath).toContain('.mnemos');
        expect(cachePath).toContain('bin');
      });
    });
  });

  describe('isCached', () => {
    let mockExistsSync;
    let mockStatSync;

    beforeEach(() => {
      mockExistsSync = jest.spyOn(fs, 'existsSync');
      mockStatSync = jest.spyOn(fs, 'statSync').mockReturnValue({
        isFile: () => true,
        size: 1024,
        mode: 0o755,
      });
    });

    afterEach(() => {
      mockExistsSync.mockRestore();
      mockStatSync.mockRestore();
    });

    test('returns true when binary exists', () => {
      mockExistsSync.mockReturnValue(true);

      const result = isCached(testVersion, testBinaryName);

      expect(result).toBe(true);
      expect(mockExistsSync).toHaveBeenCalledWith(
        expect.stringContaining(testBinaryName)
      );
    });

    test('returns false when binary does not exist', () => {
      mockExistsSync.mockReturnValue(false);

      const result = isCached(testVersion, testBinaryName);

      expect(result).toBe(false);
    });

    test('returns false for an empty cached binary', () => {
      mockExistsSync.mockReturnValue(true);
      mockStatSync.mockReturnValue({ isFile: () => true, size: 0, mode: 0o755 });

      expect(isCached(testVersion, testBinaryName)).toBe(false);
    });

    test('returns false for a non-executable Unix binary', () => {
      mockExistsSync.mockReturnValue(true);
      mockStatSync.mockReturnValue({ isFile: () => true, size: 1024, mode: 0o644 });

      expect(isCached(testVersion, testBinaryName)).toBe(process.platform === 'win32');
    });

    test('returns false when fs.existsSync throws error', () => {
      mockExistsSync.mockImplementation(() => {
        throw new Error('Permission denied');
      });

      const result = isCached(testVersion, testBinaryName);

      expect(result).toBe(false);
    });

    test('checks correct path for Unix binary', () => {
      mockExistsSync.mockReturnValue(true);

      isCached(testVersion, testBinaryName);

      const calledPath = mockExistsSync.mock.calls[0][0];
      expect(calledPath).toContain('.mnemos');
      expect(calledPath).toContain('bin');
      expect(calledPath).toContain('v1.1.10');
      expect(calledPath).toContain('mnemos');
    });

    test('checks correct path for Windows binary', () => {
      mockExistsSync.mockReturnValue(true);

      isCached(testVersion, testBinaryNameWindows);

      const calledPath = mockExistsSync.mock.calls[0][0];
      expect(calledPath).toContain('mnemos.exe');
    });

    test('handles different versions correctly', () => {
      mockExistsSync.mockReturnValue(true);

      isCached('0.5.0', testBinaryName);
      const calledPath = mockExistsSync.mock.calls[0][0];
      expect(calledPath).toContain('v0.5.0');
    });
  });

  describe('ensureCached', () => {
    let mockMkdirSync;
    let mockExistsSync;
    let mockStatSync;
    let mockDownloadFn;

    beforeEach(() => {
      mockMkdirSync = jest.spyOn(fs, 'mkdirSync').mockImplementation(() => {});
      mockExistsSync = jest.spyOn(fs, 'existsSync');
      mockStatSync = jest.spyOn(fs, 'statSync').mockReturnValue({
        isFile: () => true,
        size: 1024,
        mode: 0o755,
      });
      mockDownloadFn = jest.fn().mockResolvedValue();
    });

    afterEach(() => {
      mockMkdirSync.mockRestore();
      mockExistsSync.mockRestore();
      mockStatSync.mockRestore();
    });

    test('returns cached binary path without downloading if already cached', async () => {
      mockExistsSync.mockReturnValue(true);

      const platformInfo = {
        os: 'linux',
        arch: 'amd64',
        binaryName: testBinaryName,
      };

      const result = await ensureCached(testVersion, platformInfo, mockDownloadFn);

      expect(result).toContain(testBinaryName);
      expect(mockDownloadFn).not.toHaveBeenCalled();
      expect(mockMkdirSync).not.toHaveBeenCalled();
    });

    test('creates cache directory and downloads if not cached', async () => {
      mockExistsSync.mockReturnValue(false);

      const platformInfo = {
        os: 'linux',
        arch: 'amd64',
        binaryName: testBinaryName,
      };

      const result = await ensureCached(testVersion, platformInfo, mockDownloadFn);

      expect(mockMkdirSync).toHaveBeenCalledWith(
        expect.stringContaining('.mnemos'),
        { recursive: true }
      );
      expect(mockDownloadFn).toHaveBeenCalledWith(
        testVersion,
        platformInfo,
        expect.stringContaining('.mnemos')
      );
      expect(result).toContain(testBinaryName);
    });

    test('throws error if cache directory creation fails', async () => {
      mockExistsSync.mockReturnValue(false);
      mockMkdirSync.mockImplementation(() => {
        throw new Error('EACCES: permission denied');
      });

      const platformInfo = {
        os: 'linux',
        arch: 'amd64',
        binaryName: testBinaryName,
      };

      await expect(
        ensureCached(testVersion, platformInfo, mockDownloadFn)
      ).rejects.toThrow('Failed to create cache directory');

      expect(mockDownloadFn).not.toHaveBeenCalled();
    });

    test('error message includes cache path and action', async () => {
      mockExistsSync.mockReturnValue(false);
      mockMkdirSync.mockImplementation(() => {
        throw new Error('ENOSPC: no space left on device');
      });

      const platformInfo = {
        os: 'linux',
        arch: 'amd64',
        binaryName: testBinaryName,
      };

      await expect(
        ensureCached(testVersion, platformInfo, mockDownloadFn)
      ).rejects.toThrow('Failed to create cache directory');

      try {
        await ensureCached(testVersion, platformInfo, mockDownloadFn);
      } catch (err) {
        expect(err.message).toContain('.mnemos');
        expect(err.message).toContain('Check disk space and permissions');
      }
    });

    test('propagates download errors', async () => {
      mockExistsSync.mockReturnValue(false);
      mockDownloadFn.mockRejectedValue(new Error('Network error'));

      const platformInfo = {
        os: 'linux',
        arch: 'amd64',
        binaryName: testBinaryName,
      };

      await expect(
        ensureCached(testVersion, platformInfo, mockDownloadFn)
      ).rejects.toThrow('Network error');
    });

    test('passes correct parameters to download function', async () => {
      mockExistsSync.mockReturnValue(false);

      const platformInfo = {
        os: 'darwin',
        arch: 'arm64',
        binaryName: testBinaryName,
        archiveName: 'mnemos_darwin_arm64.tar.gz',
      };

      await ensureCached(testVersion, platformInfo, mockDownloadFn);

      expect(mockDownloadFn).toHaveBeenCalledTimes(1);
      expect(mockDownloadFn).toHaveBeenCalledWith(
        testVersion,
        platformInfo,
        expect.any(String)
      );

      const cachePath = mockDownloadFn.mock.calls[0][2];
      expect(cachePath).toContain('.mnemos');
      expect(cachePath).toContain('bin');
      expect(cachePath).toContain('v1.1.10');
    });

    test('returns correct binary path after download', async () => {
      mockExistsSync.mockReturnValue(false);

      const platformInfo = {
        os: 'windows',
        arch: 'amd64',
        binaryName: testBinaryNameWindows,
      };

      const result = await ensureCached(testVersion, platformInfo, mockDownloadFn);

      expect(result).toContain('.mnemos');
      expect(result).toContain('bin');
      expect(result).toContain('v1.1.10');
      expect(result).toContain('mnemos.exe');
    });

    test('creates cache directory with recursive option', async () => {
      mockExistsSync.mockReturnValue(false);

      const platformInfo = {
        os: 'linux',
        arch: 'amd64',
        binaryName: testBinaryName,
      };

      await ensureCached(testVersion, platformInfo, mockDownloadFn);

      expect(mockMkdirSync).toHaveBeenCalledWith(
        expect.any(String),
        { recursive: true }
      );
    });
  });

  describe('Version-based Cache Invalidation', () => {
    let mockExistsSync;
    let mockStatSync;

    beforeEach(() => {
      mockExistsSync = jest.spyOn(fs, 'existsSync');
      mockStatSync = jest.spyOn(fs, 'statSync').mockReturnValue({
        isFile: () => true,
        size: 1024,
        mode: 0o755,
      });
    });

    afterEach(() => {
      mockExistsSync.mockRestore();
      mockStatSync.mockRestore();
    });

    test('different versions use different cache paths', () => {
      const version1 = '1.0.0';
      const version2 = '1.1.0';

      const path1 = getCachePath(version1);
      const path2 = getCachePath(version2);

      expect(path1).not.toBe(path2);
      expect(path1).toContain('v1.0.0');
      expect(path2).toContain('v1.1.0');
    });

    test('isCached checks version-specific path', () => {
      mockExistsSync.mockReturnValue(true);

      isCached('1.0.0', testBinaryName);
      const path1 = mockExistsSync.mock.calls[0][0];

      mockExistsSync.mockClear();

      isCached('2.0.0', testBinaryName);
      const path2 = mockExistsSync.mock.calls[0][0];

      expect(path1).not.toBe(path2);
    });
  });

  describe('Path Construction', () => {
    test('getCachePath uses platform-appropriate separators', () => {
      const cachePath = getCachePath(testVersion);
      
      // Should use path.join which handles platform separators
      expect(cachePath).toContain('.mnemos');
      expect(cachePath).toContain('bin');
      
      // Verify it's a valid path by checking it doesn't mix separators
      const normalized = path.normalize(cachePath);
      expect(cachePath).toBe(normalized);
    });

    test('ensureCached constructs valid binary path', async () => {
      const mockExistsSync = jest.spyOn(fs, 'existsSync').mockReturnValue(true);
      const mockStatSync = jest.spyOn(fs, 'statSync').mockReturnValue({
        isFile: () => true,
        size: 1024,
        mode: 0o755,
      });

      const platformInfo = {
        os: 'linux',
        arch: 'amd64',
        binaryName: testBinaryName,
      };

      const result = await ensureCached(testVersion, platformInfo, jest.fn());

      // Should be a valid path
      const normalized = path.normalize(result);
      expect(result).toBe(normalized);

      mockExistsSync.mockRestore();
      mockStatSync.mockRestore();
    });
  });
});
