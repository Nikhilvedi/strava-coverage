#!/bin/bash

# Quick GCP Setup - Use when Cloud SQL instance already exists
# ===========================================================

set -e  # Exit on any error

echo "⚡ Strava Coverage - Quick Setup (Existing Cloud SQL)"
echo "===================================================="
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

# Set project
echo -e "${BLUE}🔧 Setting up GCP project...${NC}"
gcloud config set project $PROJECT_ID

# Verify Cloud SQL instance exists
echo -e "${BLUE}🔍 Checking Cloud SQL instance...${NC}"
if ! gcloud sql instances describe strava-coverage-db &> /dev/null; then
    echo -e "${RED}❌ Cloud SQL instance 'strava-coverage-db' not found!${NC}"
    echo -e "${YELLOW}Please run the full setup script first:${NC}"
    echo "   ./scripts/deploy-to-gcp.sh"
    exit 1
fi

echo -e "${GREEN}✅ Cloud SQL instance found${NC}"

# Create secrets (skip if already exist)
echo -e "${BLUE}🔐 Creating/updating secrets...${NC}"
echo -n "$STRAVA_CLIENT_ID" | gcloud secrets create strava-client-id --data-file=- 2>/dev/null || \
  echo -n "$STRAVA_CLIENT_ID" | gcloud secrets versions add strava-client-id --data-file=-

echo -n "$STRAVA_CLIENT_SECRET" | gcloud secrets create strava-client-secret --data-file=- 2>/dev/null || \
  echo -n "$STRAVA_CLIENT_SECRET" | gcloud secrets versions add strava-client-secret --data-file=-

# Get DB password if not creating new user
read -s -p "Enter existing database password (or new password): " DB_PASSWORD
echo ""

# Update database connection secret
DB_CONNECTION="postgres://stravauser:$DB_PASSWORD@/strava_coverage?host=/cloudsql/$PROJECT_ID:us-central1:strava-coverage-db&sslmode=disable"
echo -n "$DB_CONNECTION" | gcloud secrets create database-connection --data-file=- 2>/dev/null || \
  echo -n "$DB_CONNECTION" | gcloud secrets versions add database-connection --data-file=-

# Create service account (skip if exists)
echo -e "${BLUE}🔑 Setting up service account...${NC}"
gcloud iam service-accounts create github-actions \
  --display-name="GitHub Actions Deployment" 2>/dev/null || echo "Service account already exists"

# Grant permissions
echo -e "${BLUE}🔐 Setting up permissions...${NC}"
gcloud projects add-iam-policy-binding $PROJECT_ID \
  --member="serviceAccount:github-actions@$PROJECT_ID.iam.gserviceaccount.com" \
  --role="roles/run.admin" &> /dev/null

gcloud projects add-iam-policy-binding $PROJECT_ID \
  --member="serviceAccount:github-actions@$PROJECT_ID.iam.gserviceaccount.com" \
  --role="roles/storage.admin" &> /dev/null

gcloud projects add-iam-policy-binding $PROJECT_ID \
  --member="serviceAccount:github-actions@$PROJECT_ID.iam.gserviceaccount.com" \
  --role="roles/secretmanager.secretAccessor" &> /dev/null

gcloud projects add-iam-policy-binding $PROJECT_ID \
  --member="serviceAccount:github-actions@$PROJECT_ID.iam.gserviceaccount.com" \
  --role="roles/cloudsql.client" &> /dev/null

# Create service account key
echo -e "${BLUE}🗝️ Creating service account key...${NC}"
gcloud iam service-accounts keys create github-sa-key.json \
  --iam-account=github-actions@$PROJECT_ID.iam.gserviceaccount.com

echo ""
echo -e "${GREEN}🎉 Quick Setup Complete!${NC}"
echo "======================"
echo ""
echo -e "${BLUE}📋 GitHub Secrets to Add:${NC}"
echo ""
echo -e "${YELLOW}   GCP_PROJECT_ID${NC} = $PROJECT_ID"
echo -e "${YELLOW}   CLOUDSQL_INSTANCE${NC} = $PROJECT_ID:us-central1:strava-coverage-db"
echo ""
echo -e "${YELLOW}   GCP_SA_KEY${NC} ="
echo "$(cat github-sa-key.json)"
echo ""
echo -e "${GREEN}🚀 Ready to deploy!${NC}"

# Clean up
rm -f github-sa-key.json