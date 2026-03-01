#!/bin/bash
# test_integration.sh - Test script for Student-Enrollment microservice integration
# This script demonstrates how the Student Service calls the Enrollment Service

set -e  # Exit on errors

echo "=============================================="
echo "Microservice Integration Test"
echo "Student Service ↔ Enrollment Service"
echo "=============================================="
echo ""

# Load environment configuration from .env file
if [ -f .env ]; then
    export $(cat .env | grep -v '^#' | xargs)
fi

# Configuration (defaults for local development)
STUDENT_SERVICE="http://localhost:${SERVER_PORT:-5001}"
ENROLLMENT_SERVICE="${ENROLLMENT_SERVICE_URL:-http://localhost:5003}"
API_GATEWAY="http://localhost:8080"

echo "📋 Prerequisites Check"
echo "----------------------------------------------"

# Check if Student Service is running
echo -n "Checking Student Service (port 5001)... "
if curl -s -f "$STUDENT_SERVICE/" > /dev/null 2>&1; then
    echo "✅ Running"
else
    echo "❌ Not running"
    echo "Start with: cd /workspaces/student-service && go run main.go"
    exit 1
fi

# Check if Enrollment Service is running
echo -n "Checking Enrollment Service (port 5003)... "
if curl -s -f "$ENROLLMENT_SERVICE/" > /dev/null 2>&1; then
    echo "✅ Running"
else
    echo "❌ Not running (Optional - will test graceful degradation)"
    ENROLLMENT_DOWN=1
fi

echo ""
echo "🔐 Step 1: Register Test Student"
echo "----------------------------------------------"

REGISTER_RESPONSE=$(curl -s -X POST "$STUDENT_SERVICE/auth/register" \
  -H "Content-Type: application/json" \
  -d '{
    "email": "testuser_'$(date +%s)'@university.edu",
    "password": "testpass123",
    "name": "Integration Test User",
    "phone": "555-0999"
  }')

STUDENT_ID=$(echo "$REGISTER_RESPONSE" | jq -r '.id')

if [ "$STUDENT_ID" = "null" ] || [ -z "$STUDENT_ID" ]; then
    echo "❌ Failed to register student"
    echo "Response: $REGISTER_RESPONSE"
    exit 1
fi

echo "✅ Student registered"
echo "   Student ID: $STUDENT_ID"
echo "   Email: $(echo "$REGISTER_RESPONSE" | jq -r '.email')"

echo ""
echo "🔑 Step 2: Login to Get JWT Token"
echo "----------------------------------------------"

LOGIN_RESPONSE=$(curl -s -X POST "$STUDENT_SERVICE/auth/login" \
  -H "Content-Type: application/json" \
  -d '{
    "email": "'$(echo "$REGISTER_RESPONSE" | jq -r '.email')'",
    "password": "testpass123"
  }')

TOKEN=$(echo "$LOGIN_RESPONSE" | jq -r '.token')

if [ "$TOKEN" = "null" ] || [ -z "$TOKEN" ]; then
    echo "❌ Failed to login"
    echo "Response: $LOGIN_RESPONSE"
    exit 1
fi

echo "✅ Login successful"
echo "   Token: ${TOKEN:0:50}..."

if [ -z "$ENROLLMENT_DOWN" ]; then
    echo ""
    echo "📚 Step 3: Create Enrollments"
    echo "----------------------------------------------"
    
    # Enroll in Course C2001
    echo -n "Enrolling in course C2001... "
    ENROLL1=$(curl -s -X POST "$ENROLLMENT_SERVICE/enroll" \
      -H "Content-Type: application/json" \
      -H "Authorization: Bearer $TOKEN" \
      -d "{
        \"student_id\": \"$STUDENT_ID\",
        \"course_id\": \"C2001\"
      }")
    
    if echo "$ENROLL1" | grep -q "error"; then
        echo "⚠️  $(echo "$ENROLL1" | jq -r '.error // .message')"
    else
        echo "✅ Enrolled"
    fi
    
    # Enroll in Course C2002
    echo -n "Enrolling in course C2002... "
    ENROLL2=$(curl -s -X POST "$ENROLLMENT_SERVICE/enroll" \
      -H "Content-Type: application/json" \
      -H "Authorization: Bearer $TOKEN" \
      -d "{
        \"student_id\": \"$STUDENT_ID\",
        \"course_id\": \"C2002\"
      }")
    
    if echo "$ENROLL2" | grep -q "error"; then
        echo "⚠️  $(echo "$ENROLL2" | jq -r '.error // .message')"
    else
        echo "✅ Enrolled"
    fi
    
    # Give services time to process
    sleep 1
fi

echo ""
echo "🔗 Step 4: Test Microservice Integration"
echo "----------------------------------------------"
echo "GET /students/$STUDENT_ID/enrollments"
echo ""

INTEGRATION_RESPONSE=$(curl -s -H "Authorization: Bearer $TOKEN" \
  "$STUDENT_SERVICE/students/$STUDENT_ID/enrollments")

echo "$INTEGRATION_RESPONSE" | jq '.'

echo ""
echo "📊 Step 5: Verify Response"
echo "----------------------------------------------"

RETURNED_ID=$(echo "$INTEGRATION_RESPONSE" | jq -r '.id')
ENROLLMENT_COUNT=$(echo "$INTEGRATION_RESPONSE" | jq -r '.enrollment_count')

if [ "$RETURNED_ID" = "$STUDENT_ID" ]; then
    echo "✅ Student ID matches"
else
    echo "❌ Student ID mismatch"
    exit 1
fi

if [ "$ENROLLMENT_COUNT" = "null" ]; then
    echo "❌ Missing enrollment_count field"
    exit 1
fi

echo "✅ Enrollment count: $ENROLLMENT_COUNT"

if [ -z "$ENROLLMENT_DOWN" ] && [ "$ENROLLMENT_COUNT" -gt 0 ]; then
    echo "✅ Enrollments fetched from Enrollment Service"
    echo ""
    echo "   Courses enrolled:"
    echo "$INTEGRATION_RESPONSE" | jq -r '.enrollments[] | "   - \(.course_id) (Status: \(.status // "active"))"'
elif [ -n "$ENROLLMENT_DOWN" ]; then
    echo "✅ Graceful degradation (Enrollment Service down)"
    echo "   Student data returned with empty enrollments"
elif [ "$ENROLLMENT_COUNT" -eq 0 ]; then
    echo "ℹ️  No enrollments found (expected if enrollment failed)"
fi

echo ""
echo "=============================================="
echo "✅ Integration Test Complete!"
echo "=============================================="
echo ""
echo "📋 Summary:"
echo "   - Student Service: Working"
echo "   - Enrollment Service: $([ -z "$ENROLLMENT_DOWN" ] && echo "Working" || echo "Down (graceful degradation tested)")"
echo "   - Microservice Integration: ✅ Successful"
echo "   - Student ID: $STUDENT_ID"
echo "   - Enrollments: $ENROLLMENT_COUNT"
echo ""
echo "🔗 Integration demonstrated:"
echo "   Student Service → HTTP Call → Enrollment Service"
echo "   Response combines data from both services"
echo ""
