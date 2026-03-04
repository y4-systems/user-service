# Rate Limiter Test Suite

This folder contains comprehensive tests for the rate limiting functionality of the User Service.

## Test Files

### 1. `rate_limiter_test.go`
**Unit and integration tests** for the rate limiter using Go's testing framework.

**Tests included:**
- ✅ Rate limiter initialization
- ✅ IP-based limiter retrieval
- ✅ Rate limit enforcement
- ✅ Multiple IP address handling
- ✅ Concurrent access (thread safety)
- ✅ IP address extraction from headers (X-Forwarded-For, X-Real-IP, RemoteAddr)
- ✅ Header priority handling
- ✅ Rate limit recovery over time
- ✅ Performance benchmarks

**Run the tests:**
```bash
# Run all tests
go test -v ./test

# Run tests with coverage
go test -v -cover ./test

# Run only fast tests (skip time-dependent tests)
go test -v -short ./test

# Run benchmarks
go test -bench=. ./test

# Run a specific test
go test -v -run Test_RateLimitEnforcement ./test
```

**Expected output:**
```
=== RUN   Test_NewIPRateLimiter
--- PASS: Test_NewIPRateLimiter (0.00s)
=== RUN   Test_GetLimiter
--- PASS: Test_GetLimiter (0.00s)
=== RUN   Test_RateLimitEnforcement
--- PASS: Test_RateLimitEnforcement (0.00s)
...
PASS
ok      test    1.234s
```

---

### 2. `demo/rate_limiter_demo.go`
**Interactive demonstration** that simulates actual HTTP login requests to the running service.

**Features:**
- Makes 10 rapid login attempts
- Shows real-time results
- Tests rate limit recovery after waiting
- Color-coded output
- Connection checking

**Run the demo:**
```bash
# Make sure the service is running first!
# In one terminal:
cd /workspaces/user-service
go run main.go

# In another terminal:
cd /workspaces/user-service/test
go run demo/rate_limiter_demo.go

# Or with custom URL:
go run demo/rate_limiter_demo.go http://localhost:8080
```

**Expected output:**
```
==================================================
   Rate Limiter Demo
==================================================

🎯 Target: http://localhost:5001/auth/login
📊 Number of requests: 10
⏱️  Rate limit: 5 requests per minute per IP

Starting rapid login attempts...

Request 1: ✅ REQUEST PROCESSED (401 Unauthorized - expected)
Request 2: ❌ RATE LIMITED (429 Too Many Requests)
   Message: Too many login attempts. Please try again later.
Request 3: ❌ RATE LIMITED (429 Too Many Requests)
...
```

---

### 3. `test_rate_limiter.sh`
**Bash script** for testing rate limiting using curl (no Go required).

**Features:**
- Works on any system with bash and curl
- Tests rapid requests
- Tests rate limit recovery
- Shows statistics

**Run the script:**
```bash
# Make the script executable
chmod +x test/test_rate_limiter.sh

# Run the test
./test/test_rate_limiter.sh

# Or with custom URL:
./test/test_rate_limiter.sh http://localhost:8080
```

**Requirements:**
- bash shell
- curl command

---

## Test Scenarios

### Scenario 1: Basic Rate Limiting
**What it tests:** First request allowed, subsequent rapid requests blocked

**Expected result:**
- Request 1: ✅ Allowed (401 or 400)
- Requests 2-10: ❌ Rate limited (429)

### Scenario 2: Rate Limit Recovery
**What it tests:** Rate limit resets after time passes

**Expected result:**
- After 12+ seconds: ✅ New request allowed

### Scenario 3: Multiple IPs
**What it tests:** Different IPs have independent rate limits

**Expected result:**
- Each IP can make 5 requests per minute independently

### Scenario 4: Concurrent Access
**What it tests:** Thread-safe access to rate limiters

**Expected result:**
- No race conditions or panics under concurrent load

### Scenario 5: Header Priority
**What it tests:** IP extraction prioritizes X-Forwarded-For over X-Real-IP over RemoteAddr

**Expected result:**
- X-Forwarded-For used if present
- Falls back to X-Real-IP
- Falls back to RemoteAddr

---

## Rate Limiter Configuration

Current configuration in the User Service:

- **Rate:** 5 requests per minute per IP
- **Burst:** 1 (allows 1 immediate request)
- **Cleanup:** Every hour (removes old entries)

This means:
- Each IP can make **1 request immediately**
- Then must wait **~12 seconds** between requests
- Maximum **5 requests per minute** sustained

---

## Troubleshooting

### Problem: "Connection refused"
**Solution:** Make sure the User Service is running
```bash
cd /workspaces/user-service
go run main.go
```

### Problem: "No rate limiting detected"
**Solution:** Check that:
1. Rate limiter is initialized in main.go
2. Middleware is applied to /auth/login endpoint
3. Service is using the correct port

### Problem: Tests fail with timeout
**Solution:** 
- Skip time-dependent tests: `go test -short ./test`
- Or increase the timeout in test configuration

### Problem: "undefined: NewIPRateLimiter"
**Solution:** The test file includes its own copy of the rate limiter for isolated testing

---

## Performance Benchmarks

Run benchmarks to measure rate limiter performance:

```bash
go test -bench=. -benchmem ./test
```

**Expected results:**
- `Benchmark_GetLimiter`: ~1-2 µs per operation
- `Benchmark_Allow`: ~0.5-1 µs per operation

---

## Security Notes

The rate limiter protects against:
- ✅ **Brute force attacks** - Limits password guessing
- ✅ **Credential stuffing** - Prevents automated login attempts
- ✅ **Account enumeration** - Rate limits reduce information leakage
- ✅ **DoS attacks** - Prevents overwhelming the authentication system

