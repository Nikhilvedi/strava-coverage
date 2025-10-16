#!/bin/bash

# Check Cloud SQL Instance Status
# ===============================

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo "🔍 Cloud SQL Instance Status Checker"
echo "===================================="
echo ""

# Get project ID
if [ -z "$1" ]; then
    read -p "Enter your GCP Project ID: " PROJECT_ID
else
    PROJECT_ID=$1
fi

# Set project
gcloud config set project $PROJECT_ID &> /dev/null

# Check instance status
echo -e "${BLUE}📊 Checking strava-coverage-db status...${NC}"
echo ""

if gcloud sql instances describe strava-coverage-db &> /dev/null; then
    # Get instance details
    STATE=$(gcloud sql instances describe strava-coverage-db --format="value(state)")
    BACKEND_TYPE=$(gcloud sql instances describe strava-coverage-db --format="value(backendType)")
    TIER=$(gcloud sql instances describe strava-coverage-db --format="value(settings.tier)")
    REGION=$(gcloud sql instances describe strava-coverage-db --format="value(region)")
    
    echo -e "${GREEN}✅ Instance found!${NC}"
    echo ""
    echo "Status: $STATE"
    echo "Type: $BACKEND_TYPE"
    echo "Tier: $TIER"
    echo "Region: $REGION"
    echo ""
    
    if [ "$STATE" = "RUNNABLE" ]; then
        echo -e "${GREEN}🎉 Instance is ready to use!${NC}"
        echo ""
        echo -e "${BLUE}Next steps:${NC}"
        echo "1. Run: ./scripts/quick-setup-gcp.sh"
        echo "2. Or continue with the full setup script"
    else
        echo -e "${YELLOW}⏳ Instance is still being created...${NC}"
        echo ""
        echo -e "${BLUE}💡 You can:${NC}"
        echo "1. Wait and check again in a few minutes"
        echo "2. Monitor in Google Cloud Console"
        echo "3. Run this script again: ./scripts/check-cloudsql.sh $PROJECT_ID"
    fi
else
    echo -e "${RED}❌ Instance 'strava-coverage-db' not found${NC}"
    echo ""
    echo -e "${BLUE}💡 You need to:${NC}"
    echo "1. Run the setup script: ./scripts/deploy-to-gcp.sh"
    echo "2. Or create manually in Google Cloud Console"
fi

echo ""
echo -e "${BLUE}🔄 Recent operations:${NC}"
gcloud sql operations list --instance=strava-coverage-db --limit=3 2>/dev/null || echo "No operations found"