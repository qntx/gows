# WebSocket Client Package

A lightweight Go WebSocket client library for building WebSocket connections and handling real-time communication.

## Lock Management in the WebSocket Client

The WebSocket client employs two types of locks—`sync.RWMutex` and `sync.Mutex`—to ensure thread-safe operations in a concurrent environment. Below is a detailed yet succinct explanation of each lock, its role, and best practices for use.

### 1. `connMu` (sync.RWMutex)

- **Purpose**: Protects the connection state (`conn`, `connected`, `closed`) from race conditions during connection management and message handling.
- **Type**: Read-write mutex (`sync.RWMutex`), supporting both read (`RLock`) and write (`Lock`) operations.
- **Usage Scenarios**:
  - **Write Lock (`Lock`)**:
    - **When**: Establishing a new connection (`Connect`), closing the connection (`Close`), or resetting the connection during reconnection (`reconnect`).
    - **Why**: Ensures exclusive access when modifying the connection state (e.g., setting `conn` or `connected`).
    - **Example**: In `Connect`, `c.connMu.Lock()` guards the connection setup, with `defer c.connMu.Unlock()` ensuring release.
  - **Read Lock (`RLock`)**:
    - **When**: Checking connection status (`IsConnected`, `IsClosed`) or reading messages (`readMessages`, `send`).
    - **Why**: Allows multiple goroutines to safely inspect the state without modification.
    - **Example**: In `send`, `c.connMu.RLock()` verifies `connected` before sending a message.
- **Considerations**:
  - Always pair `Lock` with `defer Unlock()` to prevent deadlocks, especially in error paths (e.g., `Connect` failure).
  - Use `RLock` for read-only operations to maximize concurrency; avoid unnecessary `Lock` calls.
  - Non-recursive: Do not attempt to lock `connMu` again within the same goroutine while it’s already locked.

### 2. `sendMu` (sync.Mutex)

- **Purpose**: Synchronizes message sending operations to prevent concurrent writes to the WebSocket connection.
- **Type**: Standard mutex (`sync.Mutex`), supporting only exclusive locking (`Lock`).
- **Usage Scenarios**:
  - **When**: Sending any message type (`SendText`, `SendBinary`, `SendPing`, `SendPong`, `SendClose`) via the `send` method.
  - **Why**: Ensures that only one goroutine writes to `conn` at a time, avoiding data corruption or connection errors.
  - **Example**: In `send`, `c.sendMu.Lock()` protects `conn.WriteMessage`, with `defer c.sendMu.Unlock()` ensuring release.
- **Considerations**:
  - Keep the locked section minimal (e.g., only the write operation) to reduce contention.
  - Always use `defer Unlock()` to guarantee release, even if `WriteMessage` fails.
  - Non-recursive: Avoid nested locking within the same goroutine to prevent self-deadlock.

### Key Notes

- **Deadlock Prevention**: Both locks are non-recursive, meaning a goroutine cannot re-lock an already held lock. Ensure proper unlock discipline using `defer` in all write-lock scenarios.
- **Concurrency Model**: `connMu`’s read-write capability allows safe state inspection while serializing modifications. `sendMu` enforces strict sequential access for sends, complementing `connMu`’s broader scope.
- **Debugging Tip**: Enable debug mode (`WithDebug`) to log lock-related issues if unexpected behavior occurs.
