#!/bin/bash

# Generate GitHub Secrets for Deployment
# ======================================

set -e

echo "🔑 GitHub Secrets Generator for Strava Coverage"
echo "=============================================="
echo ""

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Check if gcloud is installed
if ! command -v gcloud &> /dev/null; then
    echo -e "${RED}❌ Google Cloud CLI not found. Please install it first${NC}"
    exit 1
fi

# Get project ID
read -p "Enter your GCP Project ID (not project name or number): " PROJECT_ID

# Validate project ID format
if [[ ! $PROJECT_ID =~ ^[a-z][a-z0-9-]{4,28}[a-z0-9]$ ]]; then
    echo -e "${YELLOW}⚠️ Project ID format looks unusual. Should be lowercase letters, numbers, and hyphens.${NC}"
    echo -e "${YELLOW}Example: strava-coverage-123456${NC}"
    read -p "Continue anyway? (y/n): " continue_anyway
    if [[ ! $continue_anyway =~ ^[Yy]$ ]]; then
        exit 1
    fi
fi

# Set project and verify
echo -e "${BLUE}🔧 Setting up project: $PROJECT_ID${NC}"
gcloud config set project $PROJECT_ID

# Verify we can access the project
if ! gcloud projects describe $PROJECT_ID &> /dev/null; then
    echo -e "${RED}❌ Cannot access project '$PROJECT_ID'${NC}"
    echo -e "${YELLOW}Please check:${NC}"
    echo "1. Project ID is correct (not project name or number)"
    echo "2. You have permission to access this project"
    echo "3. You are logged in: gcloud auth login"
    exit 1
fi

echo -e "${GREEN}✅ Project verified: $PROJECT_ID${NC}"
echo -e "${BLUE}🔍 Checking existing setup...${NC}"

# Check if service account exists
if ! gcloud iam service-accounts describe github-actions@$PROJECT_ID.iam.gserviceaccount.com --project=$PROJECT_ID &> /dev/null; then
    echo -e "${YELLOW}⚠️  Service account 'github-actions' not found${NC}"
    echo -e "${BLUE}Creating service account...${NC}"
    
    gcloud iam service-accounts create github-actions \
        --display-name="GitHub Actions Deployment" \
        --project=$PROJECT_ID
    
    # Grant permissions
    echo -e "${BLUE}🔐 Setting up permissions...${NC}"
    for role in "roles/run.admin" "roles/storage.admin" "roles/secretmanager.secretAccessor" "roles/cloudsql.client"; do
        echo -e "  Adding role: $role"
        gcloud projects add-iam-policy-binding $PROJECT_ID \
            --member="serviceAccount:github-actions@$PROJECT_ID.iam.gserviceaccount.com" \
            --role="$role" \
            --project=$PROJECT_ID
    done
    
    echo -e "${GREEN}✅ Service account created and configured${NC}"
else
    echo -e "${GREEN}✅ Service account already exists${NC}"
fi

# Generate new service account key
echo -e "${BLUE}🗝️ Generating fresh service account key...${NC}"

# Clean up any existing key files
rm -f github-sa-key.json

# Create new key
gcloud iam service-accounts keys create github-sa-key.json \
    --iam-account=github-actions@$PROJECT_ID.iam.gserviceaccount.com \
    --project=$PROJECT_ID

# Validate the JSON
if ! python3 -c "import json; json.load(open('github-sa-key.json'))" 2>/dev/null; then
    echo -e "${RED}❌ Generated JSON is invalid${NC}"
    exit 1
fi

echo -e "${GREEN}✅ Service account key generated and validated${NC}"

# Check Cloud SQL instance
echo -e "${BLUE}🗄️ Checking Cloud SQL instance...${NC}"
if gcloud sql instances describe strava-coverage-db --project=$PROJECT_ID &> /dev/null; then
    INSTANCE_STATUS=$(gcloud sql instances describe strava-coverage-db --project=$PROJECT_ID --format="value(state)")
    if [ "$INSTANCE_STATUS" = "RUNNABLE" ]; then
        echo -e "${GREEN}✅ Cloud SQL instance is ready${NC}"
        CLOUDSQL_INSTANCE="$PROJECT_ID:us-central1:strava-coverage-db"
    else
        echo -e "${YELLOW}⚠️  Cloud SQL instance exists but is not ready (status: $INSTANCE_STATUS)${NC}"
        CLOUDSQL_INSTANCE="$PROJECT_ID:us-central1:strava-coverage-db"
    fi
else
    echo -e "${YELLOW}⚠️  Cloud SQL instance not found. You'll need to create it first.${NC}"
    CLOUDSQL_INSTANCE="$PROJECT_ID:us-central1:strava-coverage-db"
fi

echo ""
echo -e "${GREEN}🎉 GitHub Secrets Ready!${NC}"
echo "========================"
echo ""

echo -e "${BLUE}📋 Add these to your GitHub repository secrets:${NC}"
echo ""
echo -e "${YELLOW}Repository: Backend (strava-coverage)${NC}"
echo "Settings → Secrets and variables → Actions → New repository secret"
echo ""

echo -e "${GREEN}GCP_PROJECT_ID${NC}"
echo "$PROJECT_ID"
echo ""

echo -e "${GREEN}GCP_SA_KEY${NC}"
cat github-sa-key.json
echo ""

echo -e "${GREEN}CLOUDSQL_INSTANCE${NC}"
echo "$CLOUDSQL_INSTANCE"
echo ""

echo "=================================="
echo ""
echo -e "${YELLOW}Repository: Frontend (strava-coverage-frontend)${NC}"
echo "Settings → Secrets and variables → Actions → New repository secret"
echo ""

echo -e "${GREEN}GCP_PROJECT_ID${NC}"
echo "$PROJECT_ID"
echo ""

echo -e "${GREEN}GCP_SA_KEY${NC}"
cat github-sa-key.json
echo ""

echo -e "${GREEN}NEXT_PUBLIC_API_URL${NC}"
echo "https://strava-coverage-backend-[hash].run.app"
echo -e "${YELLOW}(You'll get the exact URL after backend deployment)${NC}"
echo ""

echo "=================================="
echo ""
echo -e "${BLUE}💡 Pro Tips:${NC}"
echo "1. Copy each value exactly as shown (including all brackets and quotes)"
echo "2. Don't add extra spaces or newlines"
echo "3. The GCP_SA_KEY should start with { and end with }"
echo "4. Test the backend deployment first to get the API URL"
echo ""

# Save values to a secure file for reference
echo "# Generated secrets for $PROJECT_ID" > .github-secrets-$(date +%Y%m%d).txt
echo "GCP_PROJECT_ID=$PROJECT_ID" >> .github-secrets-$(date +%Y%m%d).txt
echo "CLOUDSQL_INSTANCE=$CLOUDSQL_INSTANCE" >> .github-secrets-$(date +%Y%m%d).txt
echo "GCP_SA_KEY contents saved in github-sa-key.json" >> .github-secrets-$(date +%Y%m%d).txt

echo -e "${GREEN}📄 Values also saved to: .github-secrets-$(date +%Y%m%d).txt${NC}"
echo -e "${YELLOW}⚠️  Remember to delete github-sa-key.json after adding to GitHub${NC}"

echo ""
echo -e "${GREEN}🚀 Ready to deploy!${NC}"
echo "1. Add secrets to GitHub repositories"
echo "2. Push code to trigger deployment"
echo "3. Check Actions tab for deployment progress"