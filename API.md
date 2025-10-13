# Strava Coverage API Documentation

Base URL: `http://localhost:8080` (development) / `https://your-domain.com` (production)

## Authentication Flow

### 1. Start OAuth Flow
```http
GET /oauth/authorize
```

Redirects user to Strava for authentication.

**Response**: Redirect to Strava OAuth page

### 2. Handle OAuth Callback
```http  
GET /oauth/callback?code={code}&scope={scope}
```

Processes Strava OAuth callback and creates/updates user.

**Parameters**:
- `code`: OAuth authorization code from Strava
- `scope`: Granted permissions

**Response**: Redirect to success page with user ID

## Activity Import

### 3. Import User Activities
```http
POST /api/import/initial/{userId}
```

Imports all historical activities for a user from Strava.

**Parameters**:
- `userId`: User ID from OAuth flow

**Response**:
```json
{
  "message": "Import started",
  "userId": 1,
  "importId": "abc123"
}
```

### 4. Check Import Status
```http
GET /api/import/status/{userId}
```

Check progress of activity import.

**Response**:
```json
{
  "status": "completed",
  "progress": {
    "total": 401,
    "imported": 401,
    "failed": 0
  },
  "startedAt": "2024-01-15T10:30:00Z",
  "completedAt": "2024-01-15T10:45:00Z"
}
```

## City Detection

### 5. Auto-Detect Cities
```http
POST /api/detection/auto-detect/{userId}
```

Automatically assigns cities to user activities based on GPS coordinates.

**Response**:
```json
{
  "message": "Auto-detection completed",
  "results": {
    "Sheffield": 258,
    "London": 12,
    "unassigned": 131
  }
}
```

## Coverage Analysis

### 6. Calculate All Coverage
```http
POST /api/multi-coverage/calculate-all/{userId}
```

Calculate coverage percentages for all user's cities.

**Response**:
```json
{
  "message": "Coverage calculation completed",
  "results": [
    {
      "cityId": 4,
      "cityName": "Sheffield",
      "coverage": 27.9,
      "totalDistance": 4184.5,
      "estimatedTotal": 14964.0
    }
  ]
}
```

### 7. Get City Coverage Details
```http
GET /api/coverage/user/{userId}/city/{cityId}
```

Get detailed coverage information for a specific city.

**Response**:
```json
{
  "cityId": 4,
  "cityName": "Sheffield", 
  "userId": 1,
  "coverage": {
    "percentage": 27.9,
    "totalDistance": 4184.5,
    "estimatedCityDistance": 14964.0,
    "activitiesCount": 258
  },
  "lastCalculated": "2024-01-15T11:00:00Z"
}
```

### 8. Get Coverage Summary
```http
GET /api/multi-coverage/user/{userId}/summary
```

Get coverage summary across all cities for a user.

**Response**:
```json
{
  "userId": 1,
  "totalActivities": 401,
  "cities": [
    {
      "cityId": 4,
      "name": "Sheffield",
      "coverage": 27.9,
      "activitiesCount": 258,
      "rank": 1
    },
    {
      "cityId": 2, 
      "name": "London",
      "coverage": 5.2,
      "activitiesCount": 12,
      "rank": 2
    }
  ],
  "lastUpdated": "2024-01-15T11:00:00Z"
}
```

## Map System (GeoJSON)

### 9. Get All Cities
```http
GET /api/maps/cities
```

Returns GeoJSON FeatureCollection of all city boundaries.

**Response**:
```json
{
  "type": "FeatureCollection",
  "features": [
    {
      "type": "Feature",
      "geometry": {
        "type": "MultiPolygon",
        "coordinates": [[[[...]]] ]
      },
      "properties": {
        "id": 4,
        "name": "Sheffield",
        "country_code": "GB"
      }
    }
  ]
}
```

### 10. Get Single City Boundary
```http
GET /api/maps/cities/{cityId}
```

Returns GeoJSON Feature for specific city boundary.

### 11. Get User Activities
```http
GET /api/maps/activities/user/{userId}
```

Returns GeoJSON FeatureCollection of user's activity paths.

**Response**:
```json
{
  "type": "FeatureCollection", 
  "features": [
    {
      "type": "Feature",
      "geometry": {
        "type": "LineString",
        "coordinates": [[-1.4659, 53.3811], [-1.4658, 53.3812]]
      },
      "properties": {
        "activity_id": 12345,
        "name": "Morning Run",
        "type": "Run",
        "distance": 5000,
        "start_date": "2024-01-15T08:00:00Z",
        "city_name": "Sheffield"
      }
    }
  ]
}
```

### 12. Get Coverage Visualization
```http
GET /api/maps/coverage/user/{userId}/city/{cityId}
```

Returns GeoJSON showing covered vs uncovered areas.

### 13. Get Map Configuration
```http
GET /api/maps/config
```

Returns complete map configuration for frontend.

**Response**:
```json
{
  "mapboxToken": null,
  "defaultCenter": [51.5074, -0.1278],
  "defaultZoom": 10,
  "tileServers": [
    {
      "name": "OpenStreetMap",
      "url": "https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png",
      "attribution": "© OpenStreetMap contributors"
    }
  ],
  "layers": {
    "cities": {
      "endpoint": "/api/maps/cities",
      "style": {
        "fillColor": "#3388ff",
        "fillOpacity": 0.2,
        "color": "#3388ff",
        "weight": 2
      }
    },
    "activities": {
      "endpoint": "/api/maps/activities/user/{userId}",
      "style": {
        "color": "#ff6b35",
        "weight": 3,
        "opacity": 0.8
      }
    }
  }
}
```

### 14. Get Map Bounds for City
```http
GET /api/maps/bounds/city/{cityId}
```

Returns optimal viewport bounds for a city.

**Response**:
```json
{
  "bounds": [
    [53.3, -1.6],  // Southwest corner
    [53.5, -1.3]   // Northeast corner  
  ],
  "center": [53.4, -1.45],
  "zoom": 11
}
```

### 15. Get Map Bounds for User Activities
```http
GET /api/maps/bounds/user/{userId}
```

Returns optimal viewport to show all user activities.

## City Management

### 16. List All Cities
```http
GET /api/cities/
```

Returns list of all available cities.

**Response**:
```json
[
  {
    "id": 4,
    "name": "Sheffield",
    "country_code": "GB",
    "created_at": "2024-01-15T09:00:00Z"
  },
  {
    "id": 2,
    "name": "London", 
    "country_code": "GB",
    "created_at": "2024-01-15T09:00:00Z"
  }
]
```

### 17. Get City Details
```http
GET /api/cities/{cityId}
```

Returns detailed information about a specific city.

**Response**:
```json
{
  "id": 4,
  "name": "Sheffield",
  "country_code": "GB",
  "boundary": "MULTIPOLYGON(...)",
  "created_at": "2024-01-15T09:00:00Z",
  "stats": {
    "totalUsers": 15,
    "totalActivities": 1247,
    "averageCoverage": 18.4
  }
}
```

## Health & Status

### 18. Health Check
```http
GET /api/health
```

Basic health check endpoint.

**Response**:
```json
{
  "status": "ok",
  "database": "connected",
  "timestamp": "2024-01-15T12:00:00Z"
}
```

## Error Responses

All endpoints return consistent error format:

```json
{
  "error": "Error description",
  "code": "ERROR_CODE",
  "timestamp": "2024-01-15T12:00:00Z"
}
```

### Common HTTP Status Codes:
- `200` - Success
- `201` - Created
- `400` - Bad Request (invalid parameters)
- `401` - Unauthorized (invalid/missing auth)
- `404` - Not Found (resource doesn't exist)
- `409` - Conflict (duplicate resource)
- `500` - Internal Server Error

### Common Error Codes:
- `USER_NOT_FOUND` - User ID doesn't exist
- `CITY_NOT_FOUND` - City ID doesn't exist  
- `IMPORT_IN_PROGRESS` - Another import is already running
- `STRAVA_API_ERROR` - Error communicating with Strava
- `DATABASE_ERROR` - Database operation failed

## Rate Limits

- Import operations: 1 per user per hour
- Coverage calculations: 10 per user per hour  
- Map endpoints: 100 per minute per IP
- General API: 1000 per hour per IP

## Webhooks (Future)

Planned webhook support for real-time activity processing:

```http
POST /api/webhooks/strava
```

Will process new activities automatically as they're uploaded to Strava.