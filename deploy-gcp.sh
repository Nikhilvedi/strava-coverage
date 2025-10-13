#!/bin/bash

# GCP Deployment Script
# Prerequisites: gcloud CLI installed and authenticated

set -e

PROJECT_ID="your-gcp-project-id"
REGION="us-central1"
SERVICE_NAME="strava-coverage"
DB_INSTANCE="strava-coverage-db"

echo "🚀 Starting GCP deployment..."

# 1. Enable required APIs
echo "📋 Enabling required APIs..."
gcloud services enable \
    cloudbuild.googleapis.com \
    run.googleapis.com \
    sqladmin.googleapis.com \
    secretmanager.googleapis.com \
    --project=$PROJECT_ID

# 2. Create Cloud SQL instance with PostGIS
echo "🗄️ Creating Cloud SQL PostgreSQL instance..."
gcloud sql instances create $DB_INSTANCE \
    --database-version=POSTGRES_15 \
    --tier=db-f1-micro \
    --region=$REGION \
    --root-password=$(openssl rand -base64 32) \
    --project=$PROJECT_ID

# Enable PostGIS extension
gcloud sql databases create strava_coverage --instance=$DB_INSTANCE --project=$PROJECT_ID

# 3. Store secrets
echo "🔐 Creating secrets..."
echo -n "$STRAVA_CLIENT_ID" | gcloud secrets create strava-client-id --data-file=- --project=$PROJECT_ID
echo -n "$STRAVA_CLIENT_SECRET" | gcloud secrets create strava-client-secret --data-file=- --project=$PROJECT_ID

# Create database connection string
DB_CONNECTION="postgres://postgres:$(gcloud sql instances describe $DB_INSTANCE --format='value(rootPassword)' --project=$PROJECT_ID)@/strava_coverage?host=/cloudsql/$PROJECT_ID:$REGION:$DB_INSTANCE"
echo -n "$DB_CONNECTION" | gcloud secrets create database-connection --data-file=- --project=$PROJECT_ID

# 4. Build and deploy container
echo "🏗️ Building container..."
gcloud builds submit --tag gcr.io/$PROJECT_ID/$SERVICE_NAME --project=$PROJECT_ID -f Dockerfile.production

echo "🚀 Deploying to Cloud Run..."
gcloud run deploy $SERVICE_NAME \
    --image gcr.io/$PROJECT_ID/$SERVICE_NAME:latest \
    --platform managed \
    --region $REGION \
    --allow-unauthenticated \
    --set-env-vars GIN_MODE=release \
    --set-secrets STRAVA_CLIENT_ID=strava-client-id:latest \
    --set-secrets STRAVA_CLIENT_SECRET=strava-client-secret:latest \
    --set-secrets DB_URL=database-connection:latest \
    --add-cloudsql-instances $PROJECT_ID:$REGION:$DB_INSTANCE \
    --memory 512Mi \
    --cpu 1 \
    --max-instances 10 \
    --project=$PROJECT_ID

# 5. Run database migrations
echo "📊 Running database migrations..."
SERVICE_URL=$(gcloud run services describe $SERVICE_NAME --region=$REGION --format='value(status.url)' --project=$PROJECT_ID)

# You'll need to run migrations manually or create a migration job
echo "✅ Deployment complete!"
echo "🌐 Service URL: $SERVICE_URL"
echo "📝 Don't forget to:"
echo "   1. Update STRAVA_REDIRECT_URI in Strava app settings to: $SERVICE_URL/oauth/callback"
echo "   2. Run database migrations"
echo "   3. Import city data"