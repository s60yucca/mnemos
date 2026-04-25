/**
 * Unit tests for binary execution module
 */

const { executeBinary } = require('../lib/execute');
const { spawn } = require('child_process');
const EventEmitter = require('events');

// Mock child_process.spawn
jest.mock('child_process');

describe('Binary Execution', () => {
  let mockChild;
  let processListeners;

  beforeEach(() => {
    // Create mock child process
    mockChild = new EventEmitter();
    mockChild.kill = jest.fn();

    // Mock spawn to return our mock child
    spawn.mockReturnValue(mockChild);

    // Track process event listeners
    processListeners = {
      SIGINT: [],
      SIGTERM: [],
    };

    // Mock process.on to track listeners
    const originalOn = process.on.bind(process);
    jest.spyOn(process, 'on').mockImplementation((event, handler) => {
      if (event === 'SIGINT' || event === 'SIGTERM') {
        processListeners[event].push(handler);
      }
      return originalOn(event, handler);
    });

    // Mock process.removeListener
    jest.spyOn(process, 'removeListener').mockImplementation((event, handler) => {
      if (event === 'SIGINT' || event === 'SIGTERM') {
        const index = processListeners[event].indexOf(handler);
        if (index > -1) {
          processListeners[event].splice(index, 1);
        }
      }
    });
  });

  afterEach(() => {
    jest.clearAllMocks();
    jest.restoreAllMocks();
  });

  describe('Process Spawning', () => {
    test('spawns child process with correct binary path', async () => {
      const binaryPath = '/path/to/mnemos';
      const args = ['serve'];

      const executePromise = executeBinary(binaryPath, args);
      
      // Simulate immediate exit
      setImmediate(() => mockChild.emit('exit', 0, null));
      
      await executePromise;

      expect(spawn).toHaveBeenCalledWith(
        binaryPath,
        args,
        expect.objectContaining({
          stdio: 'inherit',
          cwd: process.cwd(),
        })
      );
    });

    test('spawns child process with empty args when not provided', async () => {
      const binaryPath = '/path/to/mnemos';

      const executePromise = executeBinary(binaryPath);
      
      setImmediate(() => mockChild.emit('exit', 0, null));
      
      await executePromise;

      expect(spawn).toHaveBeenCalledWith(
        binaryPath,
        [],
        expect.objectContaining({
          stdio: 'inherit',
        })
      );
    });

    test('uses stdio: inherit for transparent stream forwarding', async () => {
      const binaryPath = '/path/to/mnemos';

      const executePromise = executeBinary(binaryPath, []);
      
      setImmediate(() => mockChild.emit('exit', 0, null));
      
      await executePromise;

      expect(spawn).toHaveBeenCalledWith(
        expect.any(String),
        expect.any(Array),
        expect.objectContaining({
          stdio: 'inherit',
        })
      );
    });

    test('uses current working directory', async () => {
      const binaryPath = '/path/to/mnemos';
      const currentCwd = process.cwd();

      const executePromise = executeBinary(binaryPath, []);
      
      setImmediate(() => mockChild.emit('exit', 0, null));
      
      await executePromise;

      expect(spawn).toHaveBeenCalledWith(
        expect.any(String),
        expect.any(Array),
        expect.objectContaining({
          cwd: currentCwd,
        })
      );
    });
  });

  describe('Exit Code Forwarding', () => {
    test('returns exit code 0 on successful execution', async () => {
      const binaryPath = '/path/to/mnemos';

      const executePromise = executeBinary(binaryPath, []);
      
      setImmediate(() => mockChild.emit('exit', 0, null));
      
      const exitCode = await executePromise;

      expect(exitCode).toBe(0);
    });

    test('returns exit code 1 on error', async () => {
      const binaryPath = '/path/to/mnemos';

      const executePromise = executeBinary(binaryPath, []);
      
      setImmediate(() => mockChild.emit('exit', 1, null));
      
      const exitCode = await executePromise;

      expect(exitCode).toBe(1);
    });

    test('returns exit code 127 for command not found', async () => {
      const binaryPath = '/path/to/mnemos';

      const executePromise = executeBinary(binaryPath, []);
      
      setImmediate(() => mockChild.emit('exit', 127, null));
      
      const exitCode = await executePromise;

      expect(exitCode).toBe(127);
    });

    test('returns 0 when exit code is null', async () => {
      const binaryPath = '/path/to/mnemos';

      const executePromise = executeBinary(binaryPath, []);
      
      setImmediate(() => mockChild.emit('exit', null, null));
      
      const exitCode = await executePromise;

      expect(exitCode).toBe(0);
    });
  });

  describe('Signal Handling', () => {
    test('registers SIGINT handler on process', async () => {
      const binaryPath = '/path/to/mnemos';

      const executePromise = executeBinary(binaryPath, []);
      
      // Check that SIGINT handler was registered
      expect(processListeners.SIGINT.length).toBe(1);
      
      setImmediate(() => mockChild.emit('exit', 0, null));
      await executePromise;
    });

    test('registers SIGTERM handler on process', async () => {
      const binaryPath = '/path/to/mnemos';

      const executePromise = executeBinary(binaryPath, []);
      
      // Check that SIGTERM handler was registered
      expect(processListeners.SIGTERM.length).toBe(1);
      
      setImmediate(() => mockChild.emit('exit', 0, null));
      await executePromise;
    });

    test('forwards SIGINT to child process', async () => {
      const binaryPath = '/path/to/mnemos';

      const executePromise = executeBinary(binaryPath, []);
      
      // Trigger SIGINT handler
      const sigintHandler = processListeners.SIGINT[0];
      sigintHandler();

      expect(mockChild.kill).toHaveBeenCalledWith('SIGINT');
      
      setImmediate(() => mockChild.emit('exit', 0, null));
      await executePromise;
    });

    test('forwards SIGTERM to child process', async () => {
      const binaryPath = '/path/to/mnemos';

      const executePromise = executeBinary(binaryPath, []);
      
      // Trigger SIGTERM handler
      const sigtermHandler = processListeners.SIGTERM[0];
      sigtermHandler();

      expect(mockChild.kill).toHaveBeenCalledWith('SIGTERM');
      
      setImmediate(() => mockChild.emit('exit', 0, null));
      await executePromise;
    });

    test('returns 130 when child exits with SIGINT', async () => {
      const binaryPath = '/path/to/mnemos';

      const executePromise = executeBinary(binaryPath, []);
      
      setImmediate(() => mockChild.emit('exit', null, 'SIGINT'));
      
      const exitCode = await executePromise;

      expect(exitCode).toBe(130);
    });

    test('returns 143 when child exits with SIGTERM', async () => {
      const binaryPath = '/path/to/mnemos';

      const executePromise = executeBinary(binaryPath, []);
      
      setImmediate(() => mockChild.emit('exit', null, 'SIGTERM'));
      
      const exitCode = await executePromise;

      expect(exitCode).toBe(143);
    });

    test('cleans up SIGINT handler after child exits', async () => {
      const binaryPath = '/path/to/mnemos';

      const executePromise = executeBinary(binaryPath, []);
      
      const initialSigintCount = processListeners.SIGINT.length;
      
      setImmediate(() => mockChild.emit('exit', 0, null));
      await executePromise;

      expect(process.removeListener).toHaveBeenCalledWith(
        'SIGINT',
        expect.any(Function)
      );
    });

    test('cleans up SIGTERM handler after child exits', async () => {
      const binaryPath = '/path/to/mnemos';

      const executePromise = executeBinary(binaryPath, []);
      
      setImmediate(() => mockChild.emit('exit', 0, null));
      await executePromise;

      expect(process.removeListener).toHaveBeenCalledWith(
        'SIGTERM',
        expect.any(Function)
      );
    });

    test('cleans up handlers when child exits with signal', async () => {
      const binaryPath = '/path/to/mnemos';

      const executePromise = executeBinary(binaryPath, []);
      
      setImmediate(() => mockChild.emit('exit', null, 'SIGINT'));
      await executePromise;

      expect(process.removeListener).toHaveBeenCalledWith(
        'SIGINT',
        expect.any(Function)
      );
      expect(process.removeListener).toHaveBeenCalledWith(
        'SIGTERM',
        expect.any(Function)
      );
    });
  });

  describe('Error Handling', () => {
    test('rejects with error when spawn fails', async () => {
      const binaryPath = '/path/to/nonexistent';

      const executePromise = executeBinary(binaryPath, []);
      
      const spawnError = new Error('ENOENT');
      spawnError.code = 'ENOENT';
      setImmediate(() => mockChild.emit('error', spawnError));

      await expect(executePromise).rejects.toThrow('Failed to execute binary');
      await expect(executePromise).rejects.toThrow(binaryPath);
      await expect(executePromise).rejects.toThrow('ENOENT');
    });

    test('error message includes binary path', async () => {
      const binaryPath = '/custom/path/to/mnemos';

      const executePromise = executeBinary(binaryPath, []);
      
      const spawnError = new Error('Permission denied');
      setImmediate(() => mockChild.emit('error', spawnError));

      await expect(executePromise).rejects.toThrow(binaryPath);
    });

    test('error message includes actionable advice', async () => {
      const binaryPath = '/path/to/mnemos';

      const executePromise = executeBinary(binaryPath, []);
      
      const spawnError = new Error('EACCES');
      setImmediate(() => mockChild.emit('error', spawnError));

      await expect(executePromise).rejects.toThrow('executable permissions');
    });

    test('cleans up signal handlers on spawn error', async () => {
      const binaryPath = '/path/to/mnemos';

      const executePromise = executeBinary(binaryPath, []);
      
      const spawnError = new Error('ENOENT');
      setImmediate(() => mockChild.emit('error', spawnError));

      try {
        await executePromise;
      } catch (err) {
        // Expected error
      }

      expect(process.removeListener).toHaveBeenCalledWith(
        'SIGINT',
        expect.any(Function)
      );
      expect(process.removeListener).toHaveBeenCalledWith(
        'SIGTERM',
        expect.any(Function)
      );
    });

    test('handles binary not found error (ENOENT)', async () => {
      const binaryPath = '/path/to/missing';

      const executePromise = executeBinary(binaryPath, []);
      
      const spawnError = new Error('spawn ENOENT');
      spawnError.code = 'ENOENT';
      setImmediate(() => mockChild.emit('error', spawnError));

      await expect(executePromise).rejects.toThrow('Failed to execute binary');
    });

    test('handles permission denied error (EACCES)', async () => {
      const binaryPath = '/path/to/mnemos';

      const executePromise = executeBinary(binaryPath, []);
      
      const spawnError = new Error('spawn EACCES');
      spawnError.code = 'EACCES';
      setImmediate(() => mockChild.emit('error', spawnError));

      await expect(executePromise).rejects.toThrow('Failed to execute binary');
    });
  });

  describe('Argument Forwarding', () => {
    test('forwards single argument', async () => {
      const binaryPath = '/path/to/mnemos';
      const args = ['--version'];

      const executePromise = executeBinary(binaryPath, args);
      
      setImmediate(() => mockChild.emit('exit', 0, null));
      await executePromise;

      expect(spawn).toHaveBeenCalledWith(
        binaryPath,
        ['--version'],
        expect.any(Object)
      );
    });

    test('forwards multiple arguments', async () => {
      const binaryPath = '/path/to/mnemos';
      const args = ['serve', '--port', '8080', '--host', 'localhost'];

      const executePromise = executeBinary(binaryPath, args);
      
      setImmediate(() => mockChild.emit('exit', 0, null));
      await executePromise;

      expect(spawn).toHaveBeenCalledWith(
        binaryPath,
        ['serve', '--port', '8080', '--host', 'localhost'],
        expect.any(Object)
      );
    });

    test('forwards arguments with special characters', async () => {
      const binaryPath = '/path/to/mnemos';
      const args = ['store', '--content', 'Hello "World"', '--tags', 'test,demo'];

      const executePromise = executeBinary(binaryPath, args);
      
      setImmediate(() => mockChild.emit('exit', 0, null));
      await executePromise;

      expect(spawn).toHaveBeenCalledWith(
        binaryPath,
        ['store', '--content', 'Hello "World"', '--tags', 'test,demo'],
        expect.any(Object)
      );
    });

    test('handles empty arguments array', async () => {
      const binaryPath = '/path/to/mnemos';
      const args = [];

      const executePromise = executeBinary(binaryPath, args);
      
      setImmediate(() => mockChild.emit('exit', 0, null));
      await executePromise;

      expect(spawn).toHaveBeenCalledWith(
        binaryPath,
        [],
        expect.any(Object)
      );
    });
  });
});
