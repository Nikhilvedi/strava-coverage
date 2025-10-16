#!/bin/bash

# Strava Coverage - One-Click GCP Deployment Script
# =================================================

set -e  # Exit on any error

echo "🚀 Strava Coverage - GCP Deployment Setup"
echo "=========================================="
echo ""

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Check if gcloud is installed
if ! command -v gcloud &> /dev/null; then
    echo -e "${RED}❌ Google Cloud CLI not found. Please install it first:${NC}"
    echo "   https://cloud.google.com/sdk/docs/install"
    exit 1
fi

# Get project info
echo -e "${BLUE}📋 Project Configuration${NC}"
echo "========================="
read -p "Enter your GCP Project ID: " PROJECT_ID
read -p "Enter your Strava Client ID: " STRAVA_CLIENT_ID
read -s -p "Enter your Strava Client Secret: " STRAVA_CLIENT_SECRET
echo ""
read -p "Enter a secure database password: " DB_PASSWORD
echo ""

# Set project
echo -e "${BLUE}🔧 Setting up GCP project...${NC}"
gcloud config set project $PROJECT_ID

# Enable APIs
echo -e "${BLUE}🔧 Enabling required APIs...${NC}"
gcloud services enable \
  cloudbuild.googleapis.com \
  run.googleapis.com \
  sql-component.googleapis.com \
  secretmanager.googleapis.com

# Create Cloud SQL instance
echo -e "${BLUE}🗄️ Creating Cloud SQL instance...${NC}"
if ! gcloud sql instances describe strava-coverage-db &> /dev/null; then
    gcloud sql instances create strava-coverage-db \
      --database-version=POSTGRES_15 \
      --tier=db-f1-micro \
      --region=us-central1 \
      --storage-size=10GB \
      --storage-type=SSD
    
    echo -e "${GREEN}✅ Cloud SQL instance created${NC}"
else
    echo -e "${YELLOW}⚠️  Cloud SQL instance already exists${NC}"
fi

# Create database
echo -e "${BLUE}🏗️ Creating database...${NC}"
gcloud sql databases create strava_coverage --instance=strava-coverage-db || echo "Database might already exist"

# Create user
echo -e "${BLUE}👤 Creating database user...${NC}"
gcloud sql users create stravauser \
  --instance=strava-coverage-db \
  --password=$DB_PASSWORD || echo "User might already exist"

# Create secrets
echo -e "${BLUE}🔐 Creating secrets...${NC}"
echo -n "$STRAVA_CLIENT_ID" | gcloud secrets create strava-client-id --data-file=- || \
  echo -n "$STRAVA_CLIENT_ID" | gcloud secrets versions add strava-client-id --data-file=-

echo -n "$STRAVA_CLIENT_SECRET" | gcloud secrets create strava-client-secret --data-file=- || \
  echo -n "$STRAVA_CLIENT_SECRET" | gcloud secrets versions add strava-client-secret --data-file=-

# We'll update the redirect URI after we know the Cloud Run URL
DB_CONNECTION="postgres://stravauser:$DB_PASSWORD@/strava_coverage?host=/cloudsql/$PROJECT_ID:us-central1:strava-coverage-db&sslmode=disable"
echo -n "$DB_CONNECTION" | gcloud secrets create database-connection --data-file=- || \
  echo -n "$DB_CONNECTION" | gcloud secrets versions add database-connection --data-file=-

# Create service account
echo -e "${BLUE}🔑 Creating service account...${NC}"
gcloud iam service-accounts create github-actions \
  --display-name="GitHub Actions Deployment" || echo "Service account might already exist"

# Grant permissions
echo -e "${BLUE}🔐 Setting up permissions...${NC}"
gcloud projects add-iam-policy-binding $PROJECT_ID \
  --member="serviceAccount:github-actions@$PROJECT_ID.iam.gserviceaccount.com" \
  --role="roles/run.admin"

gcloud projects add-iam-policy-binding $PROJECT_ID \
  --member="serviceAccount:github-actions@$PROJECT_ID.iam.gserviceaccount.com" \
  --role="roles/storage.admin"

gcloud projects add-iam-policy-binding $PROJECT_ID \
  --member="serviceAccount:github-actions@$PROJECT_ID.iam.gserviceaccount.com" \
  --role="roles/secretmanager.secretAccessor"

gcloud projects add-iam-policy-binding $PROJECT_ID \
  --member="serviceAccount:github-actions@$PROJECT_ID.iam.gserviceaccount.com" \
  --role="roles/cloudsql.client"

# Create service account key
echo -e "${BLUE}🗝️ Creating service account key...${NC}"
gcloud iam service-accounts keys create github-sa-key.json \
  --iam-account=github-actions@$PROJECT_ID.iam.gserviceaccount.com

echo ""
echo -e "${GREEN}🎉 GCP Setup Complete!${NC}"
echo "======================"
echo ""
echo -e "${BLUE}📋 Next Steps:${NC}"
echo "1. Add these secrets to your GitHub repository:"
echo "   - Go to: https://github.com/your-username/strava-coverage/settings/secrets/actions"
echo ""
echo -e "${YELLOW}   GCP_PROJECT_ID${NC} = $PROJECT_ID"
echo -e "${YELLOW}   CLOUDSQL_INSTANCE${NC} = $PROJECT_ID:us-central1:strava-coverage-db"
echo -e "${YELLOW}   GCP_SA_KEY${NC} = $(cat github-sa-key.json)"
echo ""
echo "2. Push your code to trigger deployment:"
echo "   git add ."
echo "   git commit -m 'Deploy to GCP'"
echo "   git push origin main"
echo ""
echo "3. After deployment, get your Cloud Run URL:"
echo "   gcloud run services list"
echo ""
echo "4. Update Strava OAuth settings with your new callback URL:"
echo "   https://your-cloud-run-url/oauth/callback"
echo ""
echo "5. Run database setup:"
echo "   ./scripts/setup-gcp-database.sh"
echo ""
echo -e "${GREEN}🚀 Your app will be live after pushing to GitHub!${NC}"

# Clean up
rm -f github-sa-key.json