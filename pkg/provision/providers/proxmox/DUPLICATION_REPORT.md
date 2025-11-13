# Code Duplication Report - Proxmox Provider

This report identifies code duplications found in the Proxmox provider implementation.

## Summary

Found **6 major duplication patterns** across multiple files that should be refactored to follow DRY principles.

---

## 1. Storage Content Checking Logic

**Severity:** High
**Files Affected:**
- `proxmox.go` (lines 129-159 in `GetStorageInfo`)
- `create.go` (lines 81-115 in `Create`)
- `create.go` (lines 218-225 in `validateStorageCapabilities`)
- `create.go` (lines 293-300 in `selectBestStorage`)
- `debug_node.go` (lines 59-65, 77-82)

**Duplication:**
The logic for checking if storage supports "iso" or "images" content is repeated in multiple places:

```go
content := strings.Split(s.Content, ",")
for _, c := range content {
    if strings.TrimSpace(c) == "iso" {
        // handle ISO storage
    }
}
```

**Recommendation:**
Create helper functions:
- `storageSupportsContent(storage StorageInfo, contentType string) bool`
- `findStorageByContent(storages []StorageInfo, contentType string) string`

---

## 2. SSH Command Execution

**Severity:** High
**Files Affected:**
- `serial.go` (lines 302-325: `runSSHCommand`)
- `dhcpd.go` (lines 41-50, 68-77, 98-102, 110-114, 156-173, 178: multiple `exec.Command("ssh", ...)` calls)

**Duplication:**
Two different approaches to SSH command execution:
1. `serial.go` has a reusable `runSSHCommand` function
2. `dhcpd.go` directly uses `exec.Command("ssh", ...)` in multiple places

**Recommendation:**
- Consolidate all SSH execution to use `runSSHCommand` from `serial.go`
- Move `runSSHCommand` to a shared location (e.g., `client.go` or new `ssh.go`)
- Update `dhcpd.go` to use the shared function

---

## 3. Task Waiting with Error Handling

**Severity:** Medium
**Files Affected:**
- `node.go` (lines 142-148: `ensureTalosISO`)
- `node.go` (lines 203-209: `ensureCloudInitISO`)
- `node.go` (lines 326-333: `createVM`)
- `node.go` (lines 349-360: `startVM`)
- `debug_node.go` (lines 135-137, 163-165, 271-277, 315-324)
- `destroy.go` (line 132)

**Duplication:**
Similar error handling pattern after `WaitForTask`:

```go
if !p.client.WaitForTask(ctx, node, taskID, timeout) {
    var task TaskStatus
    if err := p.client.Get(ctx, fmt.Sprintf("/nodes/%s/tasks/%s/status", node, taskID), &task); err == nil {
        return fmt.Errorf("task failed: status=%s, exitstatus=%s", task.Status, task.ExitStatus)
    }
    return fmt.Errorf("task failed or timed out")
}
```

**Recommendation:**
Create helper function:
- `waitForTaskWithError(ctx context.Context, node, taskID string, timeout time.Duration) error`
- Returns detailed error with task status when available

---

## 4. Host/IP Resolution Logic

**Severity:** Medium
**Files Affected:**
- `serial.go` (lines 327-343: `resolveSSHHost`)
- `dhcpd.go` (lines 187-206: `getProxmoxNodeIP`)

**Duplication:**
Both functions extract hostname/IP from Proxmox URL, but with slightly different logic:
- `resolveSSHHost` checks `PROXMOX_SSH_HOST` env var, then URL hostname, then node name
- `getProxmoxNodeIP` only extracts from URL (no env var check)

**Recommendation:**
- Consolidate into single function: `resolveProxmoxHost(config *Config, node string) string`
- Include both env var check and URL parsing
- Use consistently across both files

---

## 5. Storage Finding Logic

**Severity:** Medium
**Files Affected:**
- `proxmox.go` (lines 105-162: `GetStorageInfo`)
- `create.go` (lines 57-115: `Create` method)

**Duplication:**
Both functions:
1. Get storage list from API
2. Find storage that supports ISO uploads
3. Fallback to storage that supports images
4. Use same storage as fallback

The logic in `Create` (lines 81-115) is nearly identical to `GetStorageInfo` (lines 129-159).

**Recommendation:**
- `Create` should call `GetStorageInfo` instead of duplicating logic
- Or extract the ISO storage finding logic into a separate helper

---

## 6. ISO Existence Checking Pattern

**Severity:** Low
**Files Affected:**
- `node.go` (lines 96-119: `ensureTalosISO`)
- `node.go` (lines 176-214: `ensureCloudInitISO`)

**Duplication:**
Both functions:
1. Check if ISO exists using `CheckISOExists`
2. Log warning if check fails but continue
3. Handle upload if not exists

**Recommendation:**
- Extract common pattern into helper: `ensureISO(ctx, node, storage, filename, uploadFunc)`
- Or at least extract the existence check + warning pattern

---

## 7. VM Configuration Building (Partial)

**Severity:** Low
**Files Affected:**
- `node.go` (lines 219-308: `buildVMConfig`)
- `debug_node.go` (lines 200-223: similar parameter building)

**Duplication:**
Similar VM parameter building logic, though `debug_node.go` is simpler.

**Note:** This is acceptable duplication as `debug_node.go` is a debugging utility.

---

## Recommended Refactoring Priority

1. **High Priority:**
   - Storage content checking logic (#1)
   - SSH command execution (#2)

2. **Medium Priority:**
   - Task waiting with error handling (#3)
   - Host/IP resolution (#4)
   - Storage finding logic (#5)

3. **Low Priority:**
   - ISO existence checking pattern (#6)

---

## Files to Create/Modify

### New Helper File: `storage_helpers.go`
```go
// storageSupportsContent checks if storage supports a specific content type
func storageSupportsContent(storage StorageInfo, contentType string) bool

// findStorageByContent finds first storage that supports content type
func findStorageByContent(storages []StorageInfo, contentType string) string
```

### Modify: `client.go` or new `ssh.go`
- Move `runSSHCommand` from `serial.go`
- Add `resolveProxmoxHost` function
- Make both functions available to all files

### Modify: `client.go`
- Add `waitForTaskWithError` helper method

### Modify: `create.go`
- Use `GetStorageInfo` instead of duplicating storage finding logic
- Use `waitForTaskWithError` helper

### Modify: `dhcpd.go`
- Use shared `runSSHCommand` function
- Use shared `resolveProxmoxHost` function

---

## Impact Assessment

**Lines of Code Reduction:** ~150-200 lines
**Maintainability:** Significantly improved
**Risk:** Low (refactoring to extract helpers, not changing logic)

