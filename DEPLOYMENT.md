# Production Deployment Checklist

## Pre-Deployment Checklist

### ✅ Environment Setup
- [ ] Strava API credentials obtained
- [ ] Domain name configured (if needed)
- [ ] SSL certificates ready (if custom domain)
- [ ] Database connection string prepared
- [ ] Environment variables validated

### ✅ Database Preparation
- [ ] PostgreSQL instance with PostGIS extension
- [ ] Database created: `strava_coverage`
- [ ] Migrations applied (001, 002, 003)
- [ ] Cities data imported
- [ ] Connection tested from application

### ✅ Application Configuration
- [ ] `.env` file configured with production values
- [ ] OAuth redirect URIs updated in Strava app settings
- [ ] Database URL points to production instance
- [ ] Health check endpoint tested

## Deployment Options

### Option 1: GCP Cloud Run (Recommended)

**Benefits**: Serverless, auto-scaling, cost-effective

**Steps**:
```bash
# 1. Configure GCP project
export PROJECT_ID=your-project-id
gcloud config set project $PROJECT_ID

# 2. Enable APIs
gcloud services enable cloudbuild.googleapis.com run.googleapis.com sql-component.googleapis.com

# 3. Create Cloud SQL instance
gcloud sql instances create strava-coverage-db \
  --database-version=POSTGRES_15 \
  --tier=db-f1-micro \
  --region=europe-west2

# 4. Deploy application
./deploy-gcp.sh
```

**Environment Variables**:
- Set in Cloud Run console or via gcloud
- Use Cloud SQL Auth Proxy for database connection
- Enable Cloud SQL connections in Cloud Run

### Option 2: AWS ECS Fargate

**Benefits**: Full AWS ecosystem, managed containers

**Steps**:
```bash
# 1. Build and push image
docker build -f Dockerfile.production -t your-account.dkr.ecr.region.amazonaws.com/strava-coverage .
aws ecr get-login-password --region region | docker login --username AWS --password-stdin your-account.dkr.ecr.region.amazonaws.com
docker push your-account.dkr.ecr.region.amazonaws.com/strava-coverage

# 2. Create RDS PostgreSQL instance
aws rds create-db-instance \
  --db-instance-identifier strava-coverage-db \
  --db-instance-class db.t3.micro \
  --engine postgres \
  --master-username postgres \
  --master-user-password your-password \
  --allocated-storage 20

# 3. Register task definition
aws ecs register-task-definition --cli-input-json file://aws-ecs-task.json

# 4. Create ECS service
aws ecs create-service --cluster default --service-name strava-coverage --task-definition strava-coverage --desired-count 1
```

### Option 3: Digital Ocean App Platform

**Benefits**: Simple deployment, managed database

**Steps**:
1. Connect GitHub repository
2. Set environment variables in DO console
3. Create managed PostgreSQL database
4. Deploy via web interface

### Option 4: Docker Compose (VPS)

**Benefits**: Full control, cost-effective for high usage

**Steps**:
```bash
# 1. Copy files to server
scp docker-compose.prod.yml user@server:/opt/strava-coverage/
scp .env user@server:/opt/strava-coverage/

# 2. Deploy
ssh user@server
cd /opt/strava-coverage
docker-compose -f docker-compose.prod.yml up -d
```

## Post-Deployment Validation

### ✅ Health Checks
```bash
# Basic health check
curl https://your-domain.com/api/health

# Database connectivity
curl https://your-domain.com/api/cities

# OAuth flow
open https://your-domain.com/oauth/authorize
```

### ✅ Performance Testing
```bash
# Load test with example user
curl -X POST https://your-domain.com/api/import/initial/1
curl https://your-domain.com/api/import/status/1

# Coverage calculation
curl -X POST https://your-domain.com/api/detection/auto-detect/1
curl -X POST https://your-domain.com/api/multi-coverage/calculate-all/1
```

### ✅ Monitoring Setup
- [ ] Application logs configured
- [ ] Database monitoring enabled
- [ ] Error tracking (Sentry, etc.)
- [ ] Uptime monitoring
- [ ] Performance metrics

## Production Configuration

### Database Settings
```sql
-- Recommended PostgreSQL settings for production
ALTER SYSTEM SET shared_preload_libraries = 'postgis';
ALTER SYSTEM SET max_connections = 100;
ALTER SYSTEM SET shared_buffers = '256MB';
ALTER SYSTEM SET effective_cache_size = '1GB';
```

### Environment Variables
```bash
# Production .env template
STRAVA_CLIENT_ID=your_production_client_id
STRAVA_CLIENT_SECRET=your_production_secret
STRAVA_REDIRECT_URI=https://your-domain.com/oauth/callback
DB_URL=postgres://user:pass@db-host:5432/strava_coverage?sslmode=require
GIN_MODE=release
PORT=8080
```

## Security Checklist

### ✅ Application Security
- [ ] HTTPS enabled with valid certificates
- [ ] Database connections use SSL
- [ ] Environment variables secured (not in code)
- [ ] CORS configured for frontend domain
- [ ] Rate limiting implemented
- [ ] Input validation enabled

### ✅ Infrastructure Security  
- [ ] Database access restricted to application
- [ ] Firewall rules configured
- [ ] Secrets management (AWS Secrets Manager, etc.)
- [ ] Regular security updates scheduled
- [ ] Backup strategy implemented

## Maintenance

### Regular Tasks
- [ ] Database backups automated
- [ ] Application logs rotated
- [ ] Security patches applied
- [ ] Performance monitoring reviewed
- [ ] Cost optimization checks

### Scaling Considerations
- [ ] Database connection pooling
- [ ] Read replicas for heavy queries
- [ ] CDN for static assets
- [ ] Horizontal scaling of application instances
- [ ] Cache layer (Redis) for coverage calculations

## Troubleshooting

### Common Issues
1. **Database connection timeout**: Check security groups/firewall
2. **OAuth callback mismatch**: Verify redirect URI in Strava app
3. **PostGIS functions missing**: Ensure PostGIS extension installed
4. **Coverage calculation slow**: Check database indexes on activities table
5. **Import hanging**: Verify Strava API rate limits

### Debug Commands
```bash
# Check database connectivity
docker exec -it app-container pg_isready -h db-host -U username

# View application logs
docker logs app-container --tail=100 -f

# Test specific endpoints
curl -v https://your-domain.com/api/maps/config
```

---

## Support Contacts
- **Database Issues**: DBA team / Database provider support
- **Application Issues**: Development team / GitHub Issues  
- **Infrastructure**: DevOps team / Cloud provider support