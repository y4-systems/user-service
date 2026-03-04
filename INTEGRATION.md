# Microservice Integration Guide 🔗

## Overview

The User Service demonstrates microservice integration by calling the Enrollment Service to fetch student enrollment data. This demonstrates how microservices communicate and share data across service boundaries in a distributed architecture.

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                   API Gateway (Port 8080)                       │
│              Routes requests to User Service                    │
└────────────────────────────┬────────────────────────────────────┘
                             │
                             ▼
┌─────────────────────────────────────────────────────────────────┐
│            User Service (Port 5001)                             │
├─────────────────────────────────────────────────────────────────┤
│  Storage: MongoDB (student data)                                │
│  Endpoints:                                                     │
│  • GET  /students/{id}           → Student details             │
│  • GET  /students/{id}/enrollments → Student + Enrollments ⭐  │
│  • POST /auth/login              → Authentication             │
└────────────┬──────────────────────────────────────────────────┘
             │ HTTP Request
             │ (enrollment_client.go)
             ▼
┌─────────────────────────────────────────────────────────────────┐
│         Enrollment Service (Port 5003)                         │
├─────────────────────────────────────────────────────────────────┤
│  Storage: MongoDB (enrollment data)                            │
│  Endpoint: GET /enrollments/student/{studentId}               │
│           Returns: List of enrollments for student            │
└─────────────────────────────────────────────────────────────────┘
```

## Integration Point

### Endpoint: `GET /students/{id}/enrollments`

Retrieves a student's information along with their course enrollments by combining data from two microservices.

**Request:**
```http
GET /students/507f1f77bcf86cd799439011/enrollments
Authorization: Bearer <jwt-token>
```

**Response:**
```json
{
  "id": "507f1f77bcf86cd799439011",
  "email": "student@university.edu",
  "name": "John Doe",
  "phone": "1234567890",
  "enrollments": [
    {
      "_id": "60f7b2c9e1d3a4001f3e7a01",
      "student_id": "507f1f77bcf86cd799439011",
      "course_id": "C2001",
      "status": "active",
      "created_at": "2024-01-15T10:30:00Z",
      "updated_at": "2024-01-15T10:30:00Z"
    },
    {
      "_id": "60f7b2c9e1d3a4001f3e7a02",
      "student_id": "507f1f77bcf86cd799439011",
      "course_id": "C2002",
      "status": "active",
      "created_at": "2024-02-20T14:15:00Z"
    }
  ],
  "enrollment_count": 2
}
```