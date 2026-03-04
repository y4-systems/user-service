#!/bin/bash
# test_auth.sh - Test script for authentication endpoints (register and login)

# Don't exit on error - we want to run all tests
# set -e

echo "=============================================="
echo "Authentication API Test Suite"
echo "Testing Register and Login Endpoints"
echo "=============================================="
echo ""

# Load environment configuration from .env file
if [ -f .env ]; then
    export $(cat .env | grep -v '^#' | xargs)
fi

# Configuration
BASE_URL="http://localhost:${SERVER_PORT:-5001}"
TIMESTAMP=$(date +%s)

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Test counter
TESTS_PASSED=0
TESTS_FAILED=0

# Helper functions
pass() {
    echo -e "${GREEN}✅ PASS${NC}: $1"
    ((TESTS_PASSED++))
}

fail() {
    echo -e "${RED}❌ FAIL${NC}: $1"
    ((TESTS_FAILED++))
}

info() {
    echo -e "${YELLOW}ℹ️  INFO${NC}: $1"
}

echo "📋 Prerequisites Check"
echo "----------------------------------------------"

# Check if service is running
if ! curl -s -f "$BASE_URL/" > /dev/null 2>&1; then
    echo -e "${RED}❌ User Service is not running on $BASE_URL${NC}"
    echo "Start with: go run main.go"
    exit 1
fi

echo -e "${GREEN}✅ User Service is running${NC}"
echo ""

# Test data
TEST_EMAIL="testuser_${TIMESTAMP}@university.edu"
TEST_PASSWORD="TestPass123!"
TEST_NAME="Test User ${TIMESTAMP}"
TEST_PHONE="555-$(printf '%04d' $((RANDOM % 10000)))"

echo "🧪 Test 1: Register New Student (Valid Data)"
echo "----------------------------------------------"
info "POST $BASE_URL/auth/register"

REGISTER_RESPONSE=$(curl -s -w "\n%{http_code}" -X POST "$BASE_URL/auth/register" \
  -H "Content-Type: application/json" \
  -d "{
    \"email\": \"$TEST_EMAIL\",
    \"password\": \"$TEST_PASSWORD\",
    \"name\": \"$TEST_NAME\",
    \"phone\": \"$TEST_PHONE\"
  }")

HTTP_CODE=$(echo "$REGISTER_RESPONSE" | tail -n1)
RESPONSE_BODY=$(echo "$REGISTER_RESPONSE" | sed '$d')

echo "Response Code: $HTTP_CODE"
echo "$RESPONSE_BODY" | jq '.' 2>/dev/null || echo "$RESPONSE_BODY"

if [ "$HTTP_CODE" -eq 201 ]; then
    STUDENT_ID=$(echo "$RESPONSE_BODY" | jq -r '.id')
    if [ "$STUDENT_ID" != "null" ] && [ -n "$STUDENT_ID" ]; then
        pass "Student registered successfully"
        info "Student ID: $STUDENT_ID"
        info "Email: $TEST_EMAIL"
    else
        fail "Registration returned 201 but no ID in response"
    fi
else
    fail "Registration failed with HTTP $HTTP_CODE"
fi
echo ""

echo "🧪 Test 2: Register Duplicate Email (Should Fail)"
echo "----------------------------------------------"
info "POST $BASE_URL/auth/register"

DUPLICATE_RESPONSE=$(curl -s -w "\n%{http_code}" -X POST "$BASE_URL/auth/register" \
  -H "Content-Type: application/json" \
  -d "{
    \"email\": \"$TEST_EMAIL\",
    \"password\": \"$TEST_PASSWORD\",
    \"name\": \"Duplicate User\",
    \"phone\": \"555-9999\"
  }")

HTTP_CODE=$(echo "$DUPLICATE_RESPONSE" | tail -n1)
RESPONSE_BODY=$(echo "$DUPLICATE_RESPONSE" | sed '$d')

echo "Response Code: $HTTP_CODE"
echo "$RESPONSE_BODY" | jq '.' 2>/dev/null || echo "$RESPONSE_BODY"

if [ "$HTTP_CODE" -ge 400 ]; then
    pass "Duplicate email correctly rejected (HTTP $HTTP_CODE)"
else
    fail "Duplicate email should be rejected but got HTTP $HTTP_CODE"
fi
echo ""

echo "🧪 Test 3: Register with Missing Fields (Should Fail)"
echo "----------------------------------------------"
info "POST $BASE_URL/auth/register (missing password)"

MISSING_RESPONSE=$(curl -s -w "\n%{http_code}" -X POST "$BASE_URL/auth/register" \
  -H "Content-Type: application/json" \
  -d "{
    \"email\": \"incomplete@university.edu\",
    \"name\": \"Incomplete User\"
  }")

HTTP_CODE=$(echo "$MISSING_RESPONSE" | tail -n1)
RESPONSE_BODY=$(echo "$MISSING_RESPONSE" | sed '$d')

echo "Response Code: $HTTP_CODE"
echo "$RESPONSE_BODY" | jq '.' 2>/dev/null || echo "$RESPONSE_BODY"

if [ "$HTTP_CODE" -eq 400 ]; then
    pass "Missing fields correctly rejected with 400 Bad Request"
else
    fail "Missing fields should return 400 but got HTTP $HTTP_CODE"
fi
echo ""

echo "🧪 Test 4: Login with Valid Credentials"
echo "----------------------------------------------"
info "POST $BASE_URL/auth/login"

LOGIN_RESPONSE=$(curl -s -w "\n%{http_code}" -X POST "$BASE_URL/auth/login" \
  -H "Content-Type: application/json" \
  -d "{
    \"email\": \"$TEST_EMAIL\",
    \"password\": \"$TEST_PASSWORD\"
  }")

HTTP_CODE=$(echo "$LOGIN_RESPONSE" | tail -n1)
RESPONSE_BODY=$(echo "$LOGIN_RESPONSE" | sed '$d')

echo "Response Code: $HTTP_CODE"
echo "$RESPONSE_BODY" | jq '.' 2>/dev/null || echo "$RESPONSE_BODY"

if [ "$HTTP_CODE" -eq 200 ]; then
    TOKEN=$(echo "$RESPONSE_BODY" | jq -r '.token')
    EXPIRES_IN=$(echo "$RESPONSE_BODY" | jq -r '.expiresIn')
    
    if [ "$TOKEN" != "null" ] && [ -n "$TOKEN" ]; then
        pass "Login successful, JWT token received"
        info "Token: ${TOKEN:0:50}..."
        info "Expires In: $EXPIRES_IN"
    else
        fail "Login returned 200 but no token in response"
    fi
else
    fail "Login failed with HTTP $HTTP_CODE"
    TOKEN=""
fi
echo ""

echo "🧪 Test 5: Login with Wrong Password (Should Fail)"
echo "----------------------------------------------"
info "POST $BASE_URL/auth/login"

WRONG_PASSWORD_RESPONSE=$(curl -s -w "\n%{http_code}" -X POST "$BASE_URL/auth/login" \
  -H "Content-Type: application/json" \
  -d "{
    \"email\": \"$TEST_EMAIL\",
    \"password\": \"WrongPassword123\"
  }")

HTTP_CODE=$(echo "$WRONG_PASSWORD_RESPONSE" | tail -n1)
RESPONSE_BODY=$(echo "$WRONG_PASSWORD_RESPONSE" | sed '$d')

echo "Response Code: $HTTP_CODE"
echo "$RESPONSE_BODY" | jq '.' 2>/dev/null || echo "$RESPONSE_BODY"

if [ "$HTTP_CODE" -eq 401 ]; then
    pass "Invalid password correctly rejected with 401 Unauthorized"
else
    fail "Invalid password should return 401 but got HTTP $HTTP_CODE"
fi
echo ""

echo "🧪 Test 6: Login with Non-Existent Email (Should Fail)"
echo "----------------------------------------------"
info "POST $BASE_URL/auth/login"

NONEXISTENT_RESPONSE=$(curl -s -w "\n%{http_code}" -X POST "$BASE_URL/auth/login" \
  -H "Content-Type: application/json" \
  -d "{
    \"email\": \"nonexistent_${TIMESTAMP}@university.edu\",
    \"password\": \"SomePassword123\"
  }")

HTTP_CODE=$(echo "$NONEXISTENT_RESPONSE" | tail -n1)
RESPONSE_BODY=$(echo "$NONEXISTENT_RESPONSE" | sed '$d')

echo "Response Code: $HTTP_CODE"
echo "$RESPONSE_BODY" | jq '.' 2>/dev/null || echo "$RESPONSE_BODY"

if [ "$HTTP_CODE" -eq 401 ]; then
    pass "Non-existent email correctly rejected with 401 Unauthorized"
else
    fail "Non-existent email should return 401 but got HTTP $HTTP_CODE"
fi
echo ""

if [ -n "$TOKEN" ]; then
    echo "🧪 Test 7: Validate JWT Token"
    echo "----------------------------------------------"
    info "GET $BASE_URL/auth/validate"
    
    VALIDATE_RESPONSE=$(curl -s -w "\n%{http_code}" \
      -H "Authorization: Bearer $TOKEN" \
      "$BASE_URL/auth/validate")
    
    HTTP_CODE=$(echo "$VALIDATE_RESPONSE" | tail -n1)
    RESPONSE_BODY=$(echo "$VALIDATE_RESPONSE" | sed '$d')
    
    echo "Response Code: $HTTP_CODE"
    echo "$RESPONSE_BODY" | jq '.' 2>/dev/null || echo "$RESPONSE_BODY"
    
    if [ "$HTTP_CODE" -eq 200 ]; then
        VALIDATED_ID=$(echo "$RESPONSE_BODY" | jq -r '.id')
        VALIDATED_EMAIL=$(echo "$RESPONSE_BODY" | jq -r '.email')
        
        if [ "$VALIDATED_ID" = "$STUDENT_ID" ] && [ "$VALIDATED_EMAIL" = "$TEST_EMAIL" ]; then
            pass "Token validation successful, user details match"
        else
            fail "Token validation returned wrong user details"
        fi
    else
        fail "Token validation failed with HTTP $HTTP_CODE"
    fi
    echo ""
    
    echo "🧪 Test 8: Validate Without Token (Should Fail)"
    echo "----------------------------------------------"
    info "GET $BASE_URL/auth/validate (no Authorization header)"
    
    NO_TOKEN_RESPONSE=$(curl -s -w "\n%{http_code}" "$BASE_URL/auth/validate")
    
    HTTP_CODE=$(echo "$NO_TOKEN_RESPONSE" | tail -n1)
    RESPONSE_BODY=$(echo "$NO_TOKEN_RESPONSE" | sed '$d')
    
    echo "Response Code: $HTTP_CODE"
    echo "$RESPONSE_BODY" | jq '.' 2>/dev/null || echo "$RESPONSE_BODY"
    
    if [ "$HTTP_CODE" -eq 401 ]; then
        pass "Missing token correctly rejected with 401 Unauthorized"
    else
        fail "Missing token should return 401 but got HTTP $HTTP_CODE"
    fi
    echo ""
    
    echo "🧪 Test 9: Validate with Invalid Token (Should Fail)"
    echo "----------------------------------------------"
    info "GET $BASE_URL/auth/validate (with invalid token)"
    
    INVALID_TOKEN_RESPONSE=$(curl -s -w "\n%{http_code}" \
      -H "Authorization: Bearer invalid.token.here" \
      "$BASE_URL/auth/validate")
    
    HTTP_CODE=$(echo "$INVALID_TOKEN_RESPONSE" | tail -n1)
    RESPONSE_BODY=$(echo "$INVALID_TOKEN_RESPONSE" | sed '$d')
    
    echo "Response Code: $HTTP_CODE"
    echo "$RESPONSE_BODY" | jq '.' 2>/dev/null || echo "$RESPONSE_BODY"
    
    if [ "$HTTP_CODE" -eq 403 ]; then
        pass "Invalid token correctly rejected with 403 Forbidden"
    else
        fail "Invalid token should return 403 but got HTTP $HTTP_CODE"
    fi
    echo ""

    echo "🧪 Test 10: Rate Limiting Test"
    echo "----------------------------------------------"
    info "Testing login rate limiting (5 requests/minute per IP)"
    
    RATE_LIMIT_HIT=false
    for i in {1..7}; do
        RATE_RESPONSE=$(curl -s -w "\n%{http_code}" -X POST "$BASE_URL/auth/login" \
          -H "Content-Type: application/json" \
          -d "{
            \"email\": \"rate_test_${TIMESTAMP}@university.edu\",
            \"password\": \"test\"
          }")
        
        HTTP_CODE=$(echo "$RATE_RESPONSE" | tail -n1)
        
        if [ "$HTTP_CODE" -eq 429 ]; then
            RATE_LIMIT_HIT=true
            info "Rate limit hit on attempt $i (HTTP 429)"
            break
        fi
        
        echo -n "."
    done
    echo ""
    
    if [ "$RATE_LIMIT_HIT" = true ]; then
        pass "Rate limiting is working (429 Too Many Requests)"
    else
        fail "Rate limiting did not trigger after multiple requests"
    fi
    echo ""
fi

echo "=============================================="
echo "Test Summary"
echo "=============================================="
echo -e "${GREEN}Tests Passed: $TESTS_PASSED${NC}"
echo -e "${RED}Tests Failed: $TESTS_FAILED${NC}"
echo "Total Tests: $((TESTS_PASSED + TESTS_FAILED))"
echo ""

if [ $TESTS_FAILED -eq 0 ]; then
    echo -e "${GREEN}✅ All tests passed!${NC}"
    exit 0
else
    echo -e "${RED}❌ Some tests failed${NC}"
    exit 1
fi
