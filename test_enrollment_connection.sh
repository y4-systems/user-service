#!/bin/bash
# test_enrollment_connection.sh - Test enrollment service connection from user service

echo "=============================================="
echo "Enrollment Service Connection Test"
echo "User Service ↔ Enrollment Service (Cloud)"
echo "=============================================="
echo ""

# Load environment configuration from .env file
if [ -f .env ]; then
    export $(cat .env | grep -v '^#' | xargs)
fi

# Configuration
USER_SERVICE="http://localhost:${SERVER_PORT:-5001}"
ENROLLMENT_SERVICE_URL="${ENROLLMENT_SERVICE_URL}"

# Check if ENROLLMENT_SERVICE_URL is set
if [ -z "$ENROLLMENT_SERVICE_URL" ]; then
    echo "❌ Error: ENROLLMENT_SERVICE_URL is not set in .env file"
    echo "Please add ENROLLMENT_SERVICE_URL to your .env file"
    exit 1
fi

TIMESTAMP=$(date +%s)

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

# Test counter
TESTS_PASSED=0
TESTS_FAILED=0

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

echo "📋 Step 1: Check Service Connectivity"
echo "----------------------------------------------"

# Check User Service
echo -n "Checking User Service (localhost:5001)... "
if curl -s -f "$USER_SERVICE/" > /dev/null 2>&1; then
    echo -e "${GREEN}✅${NC}"
    pass "User Service is running"
else
    echo -e "${RED}❌${NC}"
    fail "User Service is not running"
    exit 1
fi

# Check Enrollment Service
echo -n "Checking Enrollment Service (cloud)... "
if curl -s -f "$ENROLLMENT_SERVICE_URL/" > /dev/null 2>&1; then
    echo -e "${GREEN}✅${NC}"
    pass "Enrollment Service is reachable"
else
    echo -e "${RED}❌${NC}"
    fail "Enrollment Service is not reachable"
    info "URL: $ENROLLMENT_SERVICE_URL"
fi
echo ""

echo "📝 Step 2: Register Test Student"
echo "----------------------------------------------"

TEST_EMAIL="testuser_${TIMESTAMP}@university.edu"
TEST_PASSWORD="TestPass123!"
TEST_NAME="Cloud Integration Test User"
TEST_PHONE="555-$(printf '%04d' $((RANDOM % 10000)))"

info "Registering student: $TEST_EMAIL"

REGISTER_RESPONSE=$(curl -s -X POST "$USER_SERVICE/auth/register" \
  -H "Content-Type: application/json" \
  -d "{
    \"email\": \"$TEST_EMAIL\",
    \"password\": \"$TEST_PASSWORD\",
    \"name\": \"$TEST_NAME\",
    \"phone\": \"$TEST_PHONE\"
  }")

STUDENT_ID=$(echo "$REGISTER_RESPONSE" | jq -r '.id')

if [ "$STUDENT_ID" != "null" ] && [ -n "$STUDENT_ID" ]; then
    pass "Student registered"
    info "Student ID: $STUDENT_ID"
else
    fail "Student registration failed"
    echo "$REGISTER_RESPONSE" | jq '.'
    exit 1
fi
echo ""

echo "🔑 Step 3: Login to Get JWT Token"
echo "----------------------------------------------"

info "Logging in with email: $TEST_EMAIL"

LOGIN_RESPONSE=$(curl -s -X POST "$USER_SERVICE/auth/login" \
  -H "Content-Type: application/json" \
  -d "{
    \"email\": \"$TEST_EMAIL\",
    \"password\": \"$TEST_PASSWORD\"
  }")

TOKEN=$(echo "$LOGIN_RESPONSE" | jq -r '.token')

if [ "$TOKEN" != "null" ] && [ -n "$TOKEN" ]; then
    pass "Login successful"
    info "Token obtained: ${TOKEN:0:50}..."
else
    fail "Login failed"
    echo "$LOGIN_RESPONSE" | jq '.'
    exit 1
fi
echo ""

echo "🔗 Step 4: Test Enrollment Connection Endpoint"
echo "----------------------------------------------"
info "GET /students/$STUDENT_ID/enrollments"
info "This endpoint calls Enrollment Service at: $ENROLLMENT_SERVICE_URL"
echo ""

ENROLLMENT_RESPONSE=$(curl -s -w "\n%{http_code}" \
  -H "Authorization: Bearer $TOKEN" \
  "$USER_SERVICE/students/$STUDENT_ID/enrollments")

HTTP_CODE=$(echo "$ENROLLMENT_RESPONSE" | tail -n1)
RESPONSE_BODY=$(echo "$ENROLLMENT_RESPONSE" | sed '$d')

echo "Response Code: $HTTP_CODE"
echo ""
echo "Response Body:"
echo "$RESPONSE_BODY" | jq '.' 2>/dev/null || echo "$RESPONSE_BODY"
echo ""

if [ "$HTTP_CODE" -eq 200 ]; then
    pass "Integration endpoint returned 200 OK"
    
    # Check response structure
    RETURNED_ID=$(echo "$RESPONSE_BODY" | jq -r '.id')
    ENROLLMENT_COUNT=$(echo "$RESPONSE_BODY" | jq -r '.enrollment_count')
    
    if [ "$RETURNED_ID" = "$STUDENT_ID" ]; then
        pass "Student ID matches response"
    else
        fail "Student ID mismatch"
    fi
    
    if [ "$ENROLLMENT_COUNT" != "null" ]; then
        pass "Enrollment count returned: $ENROLLMENT_COUNT"
        
        if [ "$ENROLLMENT_COUNT" -eq 0 ]; then
            info "No enrollments found (expected for new student)"
        else
            info "Student has $ENROLLMENT_COUNT enrollment(s)"
            echo "$RESPONSE_BODY" | jq '.enrollments[] | {course_id, status}'
        fi
    else
        fail "Missing enrollment_count field"
    fi
else
    fail "Integration endpoint returned HTTP $HTTP_CODE"
fi
echo ""

echo "📊 Step 5: Test Data Validation"
echo "----------------------------------------------"

# Verify all expected fields
echo "Checking response fields..."

FIELDS=(
    "id"
    "email"
    "name"
    "phone"
    "enrollments"
    "enrollment_count"
)

for field in "${FIELDS[@]}"; do
    VALUE=$(echo "$RESPONSE_BODY" | jq -r ".$field")
    if [ "$VALUE" != "null" ] && [ -n "$VALUE" ]; then
        echo -e "  ${GREEN}✓${NC} $field"
    else
        echo -e "  ${RED}✗${NC} $field (missing or null)"
    fi
done
echo ""

echo "🔍 Step 6: Connection Details"
echo "----------------------------------------------"
info "User Service Configuration:"
echo "  - Base URL: $USER_SERVICE"
echo "  - Port: 5001"
echo "  - Status: Running"
echo ""

info "Enrollment Service Configuration:"
echo "  - Base URL: $ENROLLMENT_SERVICE_URL"
echo "  - Status: $(curl -s -f "$ENROLLMENT_SERVICE_URL/" > /dev/null 2>&1 && echo "✅ Reachable" || echo "⚠️  Unreachable")"
echo ""

info "Integration Details:"
echo "  - Endpoint: GET /students/{id}/enrollments"
echo "  - Method: HTTP/REST"
echo "  - Type: Synchronous"
echo "  - Authentication: JWT Bearer Token"
echo "  - Timeout: 10 seconds"
echo ""

echo "=============================================="
echo "Test Summary"
echo "=============================================="
echo -e "${GREEN}Tests Passed: $TESTS_PASSED${NC}"
echo -e "${RED}Tests Failed: $TESTS_FAILED${NC}"
echo "Total: $((TESTS_PASSED + TESTS_FAILED))"
echo ""

if [ $TESTS_FAILED -eq 0 ]; then
    echo -e "${GREEN}✅ All tests passed!${NC}"
    echo ""
    echo "🎯 Results:"
    echo "  - User Service ✅ Working"
    echo "  - Enrollment Service ✅ Reachable"
    echo "  - Microservice Integration ✅ Successful"
    echo ""
    echo "The User Service successfully connects to the Enrollment Service"
    echo "at $ENROLLMENT_SERVICE_URL"
    exit 0
else
    echo -e "${RED}❌ Some tests failed${NC}"
    exit 1
fi
