# Strava Coverage Analysis System

A comprehensive backend system for analyzing and visualizing Strava activity coverage across cities. Built with Go, PostgreSQL + PostGIS, and Docker.

## 🎯 Features

### ✅ **Production Ready**
- **OAuth Authentication**: Complete Strava OAuth2 integration
- **Activity Import**: Bulk import of historical Strava activities  
- **Spatial Analysis**: City detection and coverage calculations
- **Map System**: 8 GeoJSON endpoints for interactive maps
- **Multi-City Support**: Coverage tracking across multiple cities
- **Real-time Processing**: Webhook integration for live updates

### 🗺️ **Configurable Map System**
- **GeoJSON APIs**: Cities, activities, coverage grids
- **Frontend Ready**: Complete configuration API
- **Multiple Tile Servers**: OpenStreetMap, Satellite imagery
- **Interactive Layers**: Customizable styling and popup fields
- **Bounds Calculation**: Automatic viewport optimization

## 🏗️ Architecture

```
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│   Frontend      │    │   Go Backend     │    │  PostgreSQL +   │
│                 │◄──►│                  │◄──►│    PostGIS      │
│ React/Next.js   │    │  Gin Framework   │    │   Docker        │
└─────────────────┘    └──────────────────┘    └─────────────────┘
                                │
                                ▼
                       ┌──────────────────┐
                       │   Strava API     │
                       │   OAuth + Data   │
                       └──────────────────┘
```

## 🚀 Quick Start

### Prerequisites
- Go 1.21+
- Docker & Docker Compose
- Strava API credentials

### 1. Clone and Setup
```bash
git clone https://github.com/Nikhilvedi/strava-coverage.git
cd strava-coverage

# Copy environment template
cp .env.example .env
# Edit .env with your Strava API credentials
```

### 2. Start Database
```bash
docker-compose up -d
```

### 3. Run Migrations
```bash
docker exec -i strava-coverage-db-1 psql -U postgres -d strava_coverage < internal/storage/migrations/001_initial_schema.sql
docker exec -i strava-coverage-db-1 psql -U postgres -d strava_coverage < internal/storage/migrations/002_coverage_schema.sql  
docker exec -i strava-coverage-db-1 psql -U postgres -d strava_coverage < internal/storage/migrations/003_import_status_schema.sql
```

### 4. Import Cities
```bash
docker exec -it strava-coverage-db-1 psql -U postgres -d strava_coverage -c "
INSERT INTO cities (name, country_code, boundary) VALUES
('London', 'GB', ST_Multi(ST_GeomFromText('POLYGON((-0.5 51.3, -0.5 51.7, 0.3 51.7, 0.3 51.3, -0.5 51.3))', 4326))),
('Sheffield', 'GB', ST_Multi(ST_GeomFromText('POLYGON((-1.6 53.3, -1.6 53.5, -1.3 53.5, -1.3 53.3, -1.6 53.3))', 4326)));
"
```

### 5. Start Backend
```bash
go run cmd/server/main.go
```

```

### 6. Test the System
```bash
# Check health
curl http://localhost:8080/api/health

# Get map configuration  
curl http://localhost:8080/api/maps/config

# Start OAuth flow
open http://localhost:8080/oauth/authorize
```

## 📊 API Endpoints

### Authentication
- `GET /oauth/authorize` - Start Strava OAuth flow
- `GET /oauth/callback` - Handle OAuth callback

### Activities & Import
- `POST /api/import/initial/:userId` - Import user's activities
- `GET /api/import/status/:userId` - Check import progress
- `POST /api/detection/auto-detect/:userId` - Assign cities to activities

### Coverage Analysis  
- `POST /api/multi-coverage/calculate-all/:userId` - Calculate all coverage
- `GET /api/coverage/user/:userId/city/:cityId` - Get city coverage
- `GET /api/multi-coverage/user/:userId/summary` - Coverage summary

### Map System (GeoJSON)
- `GET /api/maps/cities` - All cities boundaries
- `GET /api/maps/cities/:cityId` - Single city boundary  
- `GET /api/maps/activities/user/:userId` - User's activity paths
- `GET /api/maps/coverage/user/:userId/city/:cityId` - Coverage visualization
- `GET /api/maps/config` - Map configuration for frontend
- `GET /api/maps/styles` - Styling presets
- `GET /api/maps/bounds/city/:cityId` - City viewport bounds
- `GET /api/maps/bounds/user/:userId` - User activity bounds

### Cities & Management
- `GET /api/cities/` - List all cities
- `GET /api/cities/:id` - Get city details
- `POST /api/cities/` - Create new city

## 🔧 Development

### Database Schema
- **users**: Strava user accounts
- **strava_tokens**: OAuth access/refresh tokens  
- **cities**: City boundaries with PostGIS geometries
- **activities**: Imported Strava activities with paths
- **import_status**: Bulk import progress tracking

### Key Services
- **AuthService**: OAuth2 integration with Strava
- **CoverageService**: Spatial analysis and calculations
- **MapService**: GeoJSON generation for frontend
- **ImportService**: Bulk activity processing
- **CityDetectionService**: Automatic city assignment

### Testing
```bash
# Run unit tests
go test ./...

# Test specific coverage calculation
curl http://localhost:8080/api/test/coverage/1/4
```

## 🚢 Production Deployment

### Option 1: GCP Cloud Run
```bash
# Configure project
export PROJECT_ID=your-project-id
./deploy-gcp.sh
```

### Option 2: AWS ECS Fargate  
```bash
# Build and push image
docker build -f Dockerfile.production -t your-repo/strava-coverage .
docker push your-repo/strava-coverage

# Deploy ECS task
aws ecs register-task-definition --cli-input-json file://aws-ecs-task.json
```

### Option 3: Docker Compose Production
```bash
# Production deployment
docker-compose -f docker-compose.prod.yml up -d
```

## 📈 Current Status

### ✅ **Completed Features**
- OAuth authentication and user management
- Activity import from Strava API (401+ activities tested)
- City detection and assignment (258 Sheffield activities)  
- Basic coverage calculation (27.9% Sheffield coverage achieved)
- Complete map system with GeoJSON APIs
- Production deployment configurations

### 🔄 **In Progress**
- Performance optimization for large datasets
- Advanced grid-based coverage algorithms
- Real-time webhook processing
- Frontend integration documentation

### 🎯 **Roadmap**
- React/Next.js frontend with interactive maps
- Advanced analytics and leaderboards  
- Social features and sharing
- Mobile app integration
- Advanced spatial algorithms

## 🤝 Contributing

1. Fork the repository
2. Create a feature branch
3. Make changes with tests
4. Submit a pull request

## 📝 Configuration

Required environment variables:

- `STRAVA_CLIENT_ID`: Your Strava application client ID
- `STRAVA_CLIENT_SECRET`: Your Strava application client secret
- `STRAVA_REDIRECT_URI`: OAuth redirect URI (e.g., `http://localhost:8080/oauth/callback`)
- `DB_URL`: PostgreSQL connection string

## 🆘 Support

- **Issues**: GitHub Issues
- **Documentation**: See `/docs` folder
- **API Reference**: Postman collection available

---

**Built with ❤️ for the Strava community**
  - `strava/`: Strava API client

## Environment Variables

- `STRAVA_CLIENT_ID`: Your Strava API client ID
- `STRAVA_CLIENT_SECRET`: Your Strava API client secret
- `STRAVA_REDIRECT_URI`: OAuth callback URL
- `DB_URL`: PostgreSQL connection string

## License

MIT License# Thu Oct 16 23:15:09 BST 2025
